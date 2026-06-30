package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/internal/ingest"
	"finance/internal/pushnotify"
)

type nopNotifier struct{}

func (nopNotifier) OnIngestedCategory(pushnotify.Ingested) {}

func ingestService() *ingest.Service {
	return ingest.NewService(
		accountRepo(), categoryRepo(), transactionRepo(), nopNotifier{},
		&config.Finance{BaseCurrency: "UZS", TransferPairWindow: 2 * time.Minute},
		zap.NewNop(),
	)
}

func cardAccount(t *testing.T, last4 string) entities.Account {
	t.Helper()
	acc := assetAccount("UZS", 0)
	acc.Type = entities.TypeDebitCard
	acc.CardLast4 = ptr(last4)

	return mustAccount(t, acc)
}

// A Visa→Humo transfer arrives as two single legs from two sources: a Humo
// top-up (income) and a Visa withdrawal (expense). Core pairs them into one
// transfer, whichever order they land in.
func TestIngest_PairsCrossSourceTransfer(t *testing.T) {
	reset(t)
	ctx := context.Background()
	svc := ingestService()

	visa := cardAccount(t, "4853")
	humo := cardAccount(t, "8400")
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)

	// leg 1: Humo top-up via Telegram (income) — no mate yet, commits standalone
	credit, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "tg:1:201", Type: entities.TxIncome, Amount: 1_000_000,
		ToCardLast4: ptr("8400"), Date: ptr(t0), Merchant: ptr("TBC HUMO P2P"),
	})
	require.NoError(t, err)
	assert.True(t, credit.Created)
	assert.Equal(t, entities.TxIncome, credit.Transaction.Type)

	// leg 2: Visa withdrawal via the Android app (expense) — pairs with leg 1
	debit, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "sms:abc", Type: entities.TxExpense, Amount: 1_000_000,
		FromCardLast4: ptr("4853"), Date: ptr(t0.Add(20 * time.Second)), Merchant: ptr("Visa P2P"),
	})
	require.NoError(t, err)
	assert.True(t, debit.Created)

	tf := debit.Transaction
	assert.Equal(t, entities.TxTransfer, tf.Type)
	require.NotNil(t, tf.FromAccountID)
	require.NotNil(t, tf.ToAccountID)
	assert.Equal(t, visa.ID, *tf.FromAccountID)
	assert.Equal(t, humo.ID, *tf.ToAccountID)
	assert.Nil(t, tf.CategoryID, "a transfer carries no category")
	assert.Equal(t, int64(1_000_000), tf.Amount.Minor())

	// exactly one row remains: the two legs collapsed into a single transfer
	all, err := transactionRepo().List(ctx, allFilter())
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, entities.TxTransfer, all[0].Type)
}

func TestIngest_PairedLegReDeliveryIsIdempotent(t *testing.T) {
	reset(t)
	ctx := context.Background()
	svc := ingestService()

	cardAccount(t, "4853")
	cardAccount(t, "8400")
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)

	_, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "tg:1:201", Type: entities.TxIncome, Amount: 1_000_000,
		ToCardLast4: ptr("8400"), Date: ptr(t0),
	})
	require.NoError(t, err)
	merged, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "sms:abc", Type: entities.TxExpense, Amount: 1_000_000,
		FromCardLast4: ptr("4853"), Date: ptr(t0.Add(20 * time.Second)),
	})
	require.NoError(t, err)
	transferID := merged.Transaction.ID

	// re-deliver the consumed leg (the one that never got its own row)
	again, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "sms:abc", Type: entities.TxExpense, Amount: 1_000_000,
		FromCardLast4: ptr("4853"), Date: ptr(t0.Add(20 * time.Second)),
	})
	require.NoError(t, err)
	assert.False(t, again.Created, "a consumed leg must dedupe to the transfer")
	assert.Equal(t, transferID, again.Transaction.ID)

	// re-deliver the surviving leg (its external_id is kept on the transfer row)
	survivor, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "tg:1:201", Type: entities.TxIncome, Amount: 1_000_000,
		ToCardLast4: ptr("8400"), Date: ptr(t0),
	})
	require.NoError(t, err)
	assert.False(t, survivor.Created)
	assert.Equal(t, transferID, survivor.Transaction.ID)

	all, err := transactionRepo().List(ctx, allFilter())
	require.NoError(t, err)
	assert.Len(t, all, 1, "re-deliveries must not create duplicates")
}

// Two unrelated expenses of the same amount must not be glued into a transfer
// just because their amounts match; a mate needs the opposite direction.
func TestIngest_DoesNotPairSameDirectionLegs(t *testing.T) {
	reset(t)
	ctx := context.Background()
	svc := ingestService()

	cardAccount(t, "4853")
	cardAccount(t, "8400")
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)

	_, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "a", Type: entities.TxExpense, Amount: 500_000,
		FromCardLast4: ptr("4853"), Date: ptr(t0),
	})
	require.NoError(t, err)
	second, err := svc.Ingest(ctx, ingest.Command{
		ExternalID: "b", Type: entities.TxExpense, Amount: 500_000,
		FromCardLast4: ptr("8400"), Date: ptr(t0.Add(10 * time.Second)),
	})
	require.NoError(t, err)
	assert.Equal(t, entities.TxExpense, second.Transaction.Type)

	all, err := transactionRepo().List(ctx, allFilter())
	require.NoError(t, err)
	assert.Len(t, all, 2, "two same-direction expenses stay separate")
}
