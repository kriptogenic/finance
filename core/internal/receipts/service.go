package receipts

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"finance/internal/entities"
	receiptrepository "finance/internal/repositories/receipt_repository"
	"finance/pkg/proxy"
	"finance/pkg/s3"
)

// processTimeout bounds the whole async pipeline (upload + fetch + parse).
const processTimeout = 60 * time.Second

type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

// Service stores receipts and runs the async scrape+parse pipeline.
type Service struct {
	repo    receiptrepository.Repository
	storage *s3.Client
	proxy   *proxy.Client
	logger  *zap.Logger

	wg sync.WaitGroup // tracks in-flight pipelines for graceful shutdown
}

func NewService(repo receiptrepository.Repository, storage *s3.Client, proxy *proxy.Client, logger *zap.Logger) *Service {
	return &Service{repo: repo, storage: storage, proxy: proxy, logger: logger}
}

// Create validates the input, persists a pending receipt, and launches the
// background pipeline. It returns the new receipt id immediately.
func (s *Service) Create(ctx context.Context, qrURL string, photo []byte, contentType string) (uuid.UUID, error) {
	if qrURL == "" {
		return uuid.Nil, ValidationError{"qr_url is required"}
	}
	if len(photo) == 0 {
		return uuid.Nil, ValidationError{"photo is required"}
	}
	if !s.storage.Enabled() {
		return uuid.Nil, ValidationError{"receipt photo storage is not configured"}
	}
	if !s.proxy.Enabled() {
		return uuid.Nil, ValidationError{"receipt proxy is not configured"}
	}

	terminal, seq, sign, receivedAt := ParseQRParams(qrURL)
	rec := entities.Receipt{
		QRURL:      qrURL,
		Status:     entities.ReceiptPending,
		TerminalID: terminal,
		ReceiptSeq: seq,
		FiscalSign: sign,
		ReceivedAt: receivedAt,
	}
	if err := s.repo.Create(ctx, &rec); err != nil {
		return uuid.Nil, err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		bg, cancel := context.WithTimeout(context.Background(), processTimeout)
		defer cancel()
		s.process(bg, rec.ID, qrURL, photo, contentType)
	}()

	return rec.ID, nil
}

func (s *Service) process(ctx context.Context, id uuid.UUID, qrURL string, photo []byte, contentType string) {
	key := photoKey(id, time.Now().UTC())
	if err := s.storage.Upload(ctx, key, contentType, bytes.NewReader(photo)); err != nil {
		s.fail(id, "upload photo", err)

		return
	}
	if err := s.repo.SetPhotoKey(ctx, id, key); err != nil {
		s.logger.Error("set receipt photo key", zap.Error(err))
	}

	html, err := s.proxy.Fetch(ctx, qrURL)
	if err != nil {
		s.fail(id, "fetch receipt", err)

		return
	}
	if err = s.repo.SetRawHTML(ctx, id, html); err != nil {
		s.logger.Error("store receipt html", zap.Error(err))
	}

	parsed, err := ParseHTML(html)
	if err != nil {
		s.fail(id, "parse receipt", err)

		return
	}
	parsed.ID = id
	if err = s.repo.SaveParsed(ctx, &parsed); err != nil {
		s.fail(id, "save parsed", err)
	}
}

// fail marks the receipt failed using a fresh context, so a timed-out pipeline
// context doesn't also block recording the failure.
func (s *Service) fail(id uuid.UUID, stage string, cause error) {
	s.logger.Error("receipt processing failed",
		zap.String("stage", stage), zap.String("id", id.String()), zap.Error(cause))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg := stage + ": " + cause.Error()
	if err := s.repo.SetStatus(ctx, id, entities.ReceiptFailed, &msg); err != nil {
		s.logger.Error("mark receipt failed", zap.Error(err))
	}
}

// photoKey is the S3 object key: receipts/{year}/{month}/{id}.jpg.
func photoKey(id uuid.UUID, t time.Time) string {
	return fmt.Sprintf("receipts/%d/%02d/%s.jpg", t.Year(), int(t.Month()), id.String())
}

// Lifecycle drains in-flight receipt processing on shutdown.
func Lifecycle(lc fx.Lifecycle, s *Service) {
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			s.wg.Wait()

			return nil
		},
	})
}
