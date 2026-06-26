package handlers

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/receipts"
	receiptrepository "finance/internal/repositories/receipt_repository"
	"finance/pkg/money"
)

// maxPhotoBytes caps the uploaded receipt photo.
const maxPhotoBytes = 10 << 20 // 10 MiB

func (s Server) CreateReceipt(ctx context.Context, request api.CreateReceiptRequestObject) (api.CreateReceiptResponseObject, error) {
	if request.Body == nil {
		return api.CreateReceipt400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	var (
		qrURL       string
		photo       []byte
		contentType string
	)
	for {
		part, err := request.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return api.CreateReceipt400JSONResponse{BadRequestJSONResponse: badRequest("invalid multipart body")}, nil
		}

		switch part.FormName() {
		case "qr_url":
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			qrURL = strings.TrimSpace(string(b))
		case "photo":
			b, err := io.ReadAll(io.LimitReader(part, maxPhotoBytes+1))
			if err != nil {
				_ = part.Close()

				return api.CreateReceipt400JSONResponse{BadRequestJSONResponse: badRequest("could not read photo")}, nil
			}
			if len(b) > maxPhotoBytes {
				_ = part.Close()

				return api.CreateReceipt400JSONResponse{BadRequestJSONResponse: badRequest("photo too large")}, nil
			}
			photo = b
			contentType = part.Header.Get("Content-Type")
		}
		_ = part.Close()
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	id, err := s.receiptSvc.Create(ctx, qrURL, photo, contentType)
	if err != nil {
		var ve receipts.ValidationError
		if errors.As(err, &ve) {
			return api.CreateReceipt400JSONResponse{BadRequestJSONResponse: badRequest(ve.Error())}, nil
		}
		s.logger.Error("create receipt", zap.Error(err))

		return nil, err
	}

	return api.CreateReceipt201JSONResponse{Id: id, Status: api.Pending}, nil
}

func (s Server) GetReceipt(ctx context.Context, request api.GetReceiptRequestObject) (api.GetReceiptResponseObject, error) {
	rec, err := s.receipts.Get(ctx, request.Id)
	if errors.Is(err, receiptrepository.ErrNotFound) {
		return api.GetReceipt404JSONResponse{NotFoundJSONResponse: notFound("receipt not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetReceipt200JSONResponse(toAPIReceipt(*rec)), nil
}

func (s Server) ListReceipts(ctx context.Context, request api.ListReceiptsRequestObject) (api.ListReceiptsResponseObject, error) {
	page, limit := 1, 20
	if request.Params.Page != nil {
		page = *request.Params.Page
	}
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	recs, err := s.receipts.List(ctx, page, limit)
	if err != nil {
		return nil, err
	}

	out := make([]api.Receipt, 0, len(recs))
	for i := range recs {
		out = append(out, toAPIReceipt(recs[i]))
	}

	return api.ListReceipts200JSONResponse{Receipts: out}, nil
}

func toAPIReceipt(r entities.Receipt) api.Receipt {
	out := api.Receipt{
		Id:              r.ID,
		QrUrl:           r.QRURL,
		Status:          api.ReceiptStatus(r.Status),
		Error:           nstr(r.Error),
		TerminalId:      nstr(r.TerminalID),
		ReceiptSeq:      nint(r.ReceiptSeq),
		FiscalSign:      nstr(r.FiscalSign),
		ReceivedAt:      ntime(r.ReceivedAt),
		ReceiptType:     nstr(r.ReceiptType),
		MerchantName:    nstr(r.MerchantName),
		MerchantTin:     nstr(r.MerchantTIN),
		MerchantAddress: nstr(r.MerchantAddress),
		DeviceName:      nstr(r.DeviceName),
		SerialNumber:    nstr(r.SerialNumber),
		CardType:        nstr(r.CardType),
		MerchantLat:     nstr(r.MerchantLat),
		MerchantLng:     nstr(r.MerchantLng),
		PaidCash:        moneyPtr(r.PaidCash),
		PaidCard:        moneyPtr(r.PaidCard),
		TotalAmount:     moneyPtr(r.TotalAmount),
		TotalVat:        moneyPtr(r.TotalVAT),
		PhotoKey:        nstr(r.PhotoKey),
		ScrapedAt:       ntime(r.ScrapedAt),
		CreatedAt:       r.CreatedAt,
		Items:           make([]api.ReceiptItem, 0, len(r.Items)),
	}

	for _, it := range r.Items {
		out.Items = append(out.Items, api.ReceiptItem{
			Name:         it.Name,
			Quantity:     it.Quantity,
			Price:        money.New(it.Price, entities.ReceiptCurrency),
			VatAmount:    money.New(it.VATAmount, entities.ReceiptCurrency),
			VatRate:      it.VATRate,
			Discount:     moneyPtr(it.Discount),
			Other:        moneyPtr(it.Other),
			Barcode:      nstr(it.Barcode),
			IkpuCode:     nstr(it.IKPUCode),
			IkpuName:     nstr(it.IKPUName),
			Unit:         nstr(it.Unit),
			MarkingCode:  nstr(it.MarkingCode),
			ConsignorTin: nstr(it.ConsignorTIN),
		})
	}

	return out
}

func moneyPtr(amount int64) *money.Money {
	m := money.New(amount, entities.ReceiptCurrency)

	return &m
}

func nstr(p *string) nullable.Nullable[string] {
	if p == nil {
		return nullable.Nullable[string]{}
	}

	return nullable.NewNullableWithValue(*p)
}

func nint(p *int) nullable.Nullable[int] {
	if p == nil {
		return nullable.Nullable[int]{}
	}

	return nullable.NewNullableWithValue(*p)
}

func ntime(p *time.Time) nullable.Nullable[time.Time] {
	if p == nil {
		return nullable.Nullable[time.Time]{}
	}

	return nullable.NewNullableWithValue(*p)
}
