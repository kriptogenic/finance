package telegram

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/mdp/qrterminal/v3"
)

func (s *Source) authenticateQR(ctx context.Context, client *telegram.Client, dispatcher tg.UpdateDispatcher) error {
	loggedIn := qrlogin.OnLoginToken(dispatcher)
	_, err := client.QR().Auth(ctx, loggedIn, func(ctx context.Context, token qrlogin.Token) error {
		fmt.Fprintln(s.out, "Scan this QR code in Telegram → Settings → Devices → Link Desktop Device:")
		qrterminal.GenerateHalfBlock(token.URL(), qrterminal.L, s.out)
		return nil
	})
	if err == nil {
		return nil
	}
	if !tgerr.Is(err, "SESSION_PASSWORD_NEEDED") {
		return err
	}
	password, perr := newTermAuth(s.in, s.out, s.cfg.Phone).Password(ctx)
	if perr != nil {
		return perr
	}
	if _, err := client.Auth().Password(ctx, password); err != nil {
		return fmt.Errorf("2fa password: %w", err)
	}
	return nil
}

type termAuth struct {
	in    *bufio.Reader
	out   io.Writer
	phone string
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

	return a.prompt("Enter 2FA password: ")
}

func (a termAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return a.prompt("Enter login code (sent via Telegram): ")
}

func (a termAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	return fmt.Errorf("unexpected terms-of-service prompt; refusing")
}

func (a termAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign-up is not supported; log into an existing account")
}

var _ auth.UserAuthenticator = termAuth{}

func defaultAuthIO() (io.Reader, io.Writer) { return os.Stdin, os.Stdout }
