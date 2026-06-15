// Package telegram is the bot's Telegram transport (§3): an MTProto user session
// (gotd, not the Bot API) that authenticates once, persists the session, resolves
// the single source bot's peer, and fetches message history. It is deliberately
// read-only — it never sends a Telegram message — which is the lowest-risk
// userbot behaviour (§2).
//
// It owns no business logic: Run hands the caller a Fetcher and lets the
// orchestrator drive the poll cadence and processing, so a slow ingest endpoint
// can never stall inside this package (§12).
package telegram

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/log/logzap"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// Config is the transport's slice of the bot config (§9).
type Config struct {
	APIID        int
	APIHash      string
	SessionFile  string
	SourceBot    string // username (with or without @); numeric IDs are not supported
	Phone        string // optional; empty → prompt interactively
	PollInterval time.Duration
}

// Message is a fetched message, decoupled from gotd's tg types so downstream
// stages don't depend on the Telegram library.
type Message struct {
	ID   int
	Date time.Time
	Text string
}

// Fetcher returns messages newer than a watermark. The orchestrator calls it on
// its own schedule.
type Fetcher interface {
	// FetchNewer returns messages with ID strictly greater than sinceID, in
	// ascending ID order (oldest first), so they can be processed in sequence.
	FetchNewer(ctx context.Context, sinceID int) ([]Message, error)
	// ChatID is the source chat's ID, used to build the tg:<chat>:<msg>
	// idempotency key (§7). It is constant for the single watched chat.
	ChatID() int64
}

// Source manages the Telegram connection.
type Source struct {
	cfg Config
	log *zap.Logger
	in  io.Reader
	out io.Writer
}

// New builds a Source. authIn/authOut drive the one-time interactive login; pass
// nil to use stdin/stdout.
func New(cfg Config, log *zap.Logger, authIn io.Reader, authOut io.Writer) *Source {
	if authIn == nil || authOut == nil {
		di, do := defaultAuthIO()
		if authIn == nil {
			authIn = di
		}
		if authOut == nil {
			authOut = do
		}
	}
	return &Source{cfg: cfg, log: log, in: authIn, out: authOut}
}

// Run connects, authenticates (interactively only if the session is missing or
// expired), resolves the source peer once, then invokes run with a Fetcher bound
// to that peer. The Telegram connection stays open for the duration of run and
// closes when it returns. A flood-wait middleware transparently honours
// FLOOD_WAIT throttling (§3).
func (s *Source) Run(ctx context.Context, run func(ctx context.Context, f Fetcher) error) error {
	waiter := floodwait.NewSimpleWaiter()
	client := telegram.NewClient(s.cfg.APIID, s.cfg.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: s.cfg.SessionFile},
		Logger:         logzap.New(s.log.Named("gotd")),
		Middlewares:    []telegram.Middleware{waiter},
	})

	return client.Run(ctx, func(ctx context.Context) error {
		if err := s.authenticate(ctx, client); err != nil {
			return fmt.Errorf("authenticate: %w", err)
		}
		peer, err := s.resolvePeer(ctx, client.API())
		if err != nil {
			return fmt.Errorf("resolve source peer %q: %w", s.cfg.SourceBot, err)
		}
		s.log.Info("connected; source peer resolved", zap.String("source_bot", s.cfg.SourceBot))
		f := &fetcher{api: client.API(), peer: peer, chatID: peerID(peer), log: s.log}
		return run(ctx, f)
	})
}

// authenticate logs in if necessary using the interactive authenticator. On a
// persisted session this is a no-op (§3).
func (s *Source) authenticate(ctx context.Context, client *telegram.Client) error {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return fmt.Errorf("auth status: %w", err)
	}
	if status.Authorized {
		return nil
	}
	flow := auth.NewFlow(newTermAuth(s.in, s.out, s.cfg.Phone), auth.SendCodeOptions{})
	return client.Auth().IfNecessary(ctx, flow)
}

// resolvePeer resolves the source bot's username to an input peer with its access
// hash, once (§3). Numeric IDs are not supported because resolving them needs an
// access hash the bot doesn't have up front.
func (s *Source) resolvePeer(ctx context.Context, api *tg.Client) (tg.InputPeerClass, error) {
	username := strings.TrimPrefix(strings.TrimSpace(s.cfg.SourceBot), "@")
	if username == "" {
		return nil, fmt.Errorf("SOURCE_BOT is empty")
	}
	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return nil, err
	}
	for _, u := range resolved.Users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
	}
	return nil, fmt.Errorf("username %q did not resolve to a user", username)
}

// fetcher fetches history for one resolved peer.
type fetcher struct {
	api    *tg.Client
	peer   tg.InputPeerClass
	chatID int64
	log    *zap.Logger
}

func (f *fetcher) ChatID() int64 { return f.chatID }

// peerID extracts the numeric ID from a resolved input peer for use as the
// chat_id in idempotency keys.
func peerID(p tg.InputPeerClass) int64 {
	switch v := p.(type) {
	case *tg.InputPeerUser:
		return v.UserID
	case *tg.InputPeerChat:
		return v.ChatID
	case *tg.InputPeerChannel:
		return v.ChannelID
	default:
		return 0
	}
}

const historyPageLimit = 100

// FetchNewer pages backwards through history (newest first) collecting every
// message with ID > sinceID, then returns them oldest-first. MinID makes the
// server exclude anything at or below the watermark, so this naturally
// backfills only what was missed since the last run (§3).
func (f *fetcher) FetchNewer(ctx context.Context, sinceID int) ([]Message, error) {
	var out []Message
	offsetID := 0 // 0 = start from the newest message

	for {
		resp, err := f.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     f.peer,
			MinID:    sinceID,
			OffsetID: offsetID,
			Limit:    historyPageLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("get history (offset %d): %w", offsetID, err)
		}
		msgs := messagesOf(resp)
		if len(msgs) == 0 {
			break
		}
		for _, m := range msgs {
			msg, ok := m.(*tg.Message)
			if !ok { // skip service messages (joins, etc.)
				continue
			}
			out = append(out, Message{
				ID:   msg.ID,
				Date: time.Unix(int64(msg.Date), 0),
				Text: msg.Message,
			})
		}
		// Page older: the next request starts below the oldest ID in this page.
		offsetID = msgs[len(msgs)-1].GetID()
		if len(msgs) < historyPageLimit {
			break
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// messagesOf extracts the message list from any of the history response variants.
func messagesOf(resp tg.MessagesMessagesClass) []tg.MessageClass {
	switch m := resp.(type) {
	case *tg.MessagesMessages:
		return m.Messages
	case *tg.MessagesMessagesSlice:
		return m.Messages
	case *tg.MessagesChannelMessages:
		return m.Messages
	default:
		return nil
	}
}
