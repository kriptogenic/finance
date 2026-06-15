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

type Config struct {
	APIID        int
	APIHash      string
	SessionFile  string
	SourceBot    string
	Phone        string
	PollInterval time.Duration
}

type Message struct {
	ID   int
	Date time.Time
	Text string
}

type Fetcher interface {
	FetchNewer(ctx context.Context, sinceID int) ([]Message, error)

	ChatID() int64
}

type Source struct {
	cfg Config
	log *zap.Logger
	in  io.Reader
	out io.Writer
}

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

type fetcher struct {
	api    *tg.Client
	peer   tg.InputPeerClass
	chatID int64
	log    *zap.Logger
}

func (f *fetcher) ChatID() int64 { return f.chatID }

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

func (f *fetcher) FetchNewer(ctx context.Context, sinceID int) ([]Message, error) {
	var out []Message
	offsetID := 0

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
			if !ok {
				continue
			}
			out = append(out, Message{
				ID:   msg.ID,
				Date: time.Unix(int64(msg.Date), 0),
				Text: msg.Message,
			})
		}

		offsetID = msgs[len(msgs)-1].GetID()
		if len(msgs) < historyPageLimit {
			break
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

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
