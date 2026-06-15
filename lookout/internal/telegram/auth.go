package telegram

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// termAuth is a one-time interactive UserAuthenticator: it prompts on out and
// reads the phone number, login code, and 2FA password from in (stdin) (§3).
// After the first successful login the session file makes this unnecessary, so
// the bot reconnects silently. It deliberately refuses sign-up — this account
// must already exist.
type termAuth struct {
	in    *bufio.Reader
	out   io.Writer
	phone string // optional pre-seed from env; empty → prompt
}

func newTermAuth(in io.Reader, out io.Writer, phone string) termAuth {
	return termAuth{in: bufio.NewReader(in), out: out, phone: phone}
}

func (a termAuth) prompt(label string) (string, error) {
	fmt.Fprint(a.out, label)
	line, err := a.in.ReadString('\n')
	if err != nil && !(err == io.EOF && line != "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a termAuth) Phone(_ context.Context) (string, error) {
	if a.phone != "" {
		return a.phone, nil
	}
	return a.prompt("Enter phone number (international, e.g. +99890…): ")
}

func (a termAuth) Password(_ context.Context) (string, error) {
	// Note: this echoes input. For a one-time CLI login that is acceptable; the
	// password is never logged or persisted (only the derived session is).
	return a.prompt("Enter 2FA password: ")
}

func (a termAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return a.prompt("Enter login code (sent via Telegram): ")
}

// AcceptTermsOfService should never fire for an existing account; refuse rather
// than silently accept new terms.
func (a termAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	return fmt.Errorf("unexpected terms-of-service prompt; refusing")
}

// SignUp is disallowed — the bot logs into an existing account only.
func (a termAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign-up is not supported; log into an existing account")
}

var _ auth.UserAuthenticator = termAuth{}

// defaultAuthIO is the interactive IO used in production (stdin/stdout).
func defaultAuthIO() (io.Reader, io.Writer) { return os.Stdin, os.Stdout }
