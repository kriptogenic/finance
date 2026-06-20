package accountrepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/database"
)

var (
	ErrNotFound = errors.New("account not found")
	ErrInUse    = errors.New("account has transactions")
)

type Repository interface {
	Create(ctx context.Context, acc *entities.Account) error
	List(ctx context.Context, includeArchived bool) ([]entities.Account, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Account, error)
	// ByCardLast4 resolves the account that owns a card (external ingest routing).
	ByCardLast4(ctx context.Context, last4 string) (*entities.Account, error)
	Update(ctx context.Context, acc *entities.Account) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Balances returns the derived balance (account currency, minor units) for
	// every account, keyed by id.
	Balances(ctx context.Context) (map[uuid.UUID]int64, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

// include_in_net_worth is scanned last and inverted into ExcludedFromNetWorth.
const accountColumns = `id, name, kind, type, currency, opening_balance, archived, created_at,
	interest_rate, term_months, maturity_date, capitalization,
	credit_limit, principal, start_date, payment_day, card_last4, include_in_net_worth`

func scanAccount(row pgx.Row) (entities.Account, error) {
	var (
		a       entities.Account
		include bool
	)
	err := row.Scan(
		&a.ID, &a.Name, &a.Kind, &a.Type, &a.Currency, &a.OpeningBalance, &a.Archived, &a.CreatedAt,
		&a.InterestRate, &a.TermMonths, &a.MaturityDate, &a.Capitalization,
		&a.CreditLimit, &a.Principal, &a.StartDate, &a.PaymentDay, &a.CardLast4, &include,
	)
	a.ExcludedFromNetWorth = !include

	return a, err
}

func (r repository) Create(ctx context.Context, acc *entities.Account) error {
	const query = `
		INSERT INTO accounts
			(name, kind, type, currency, opening_balance, archived,
			 interest_rate, term_months, maturity_date, capitalization,
			 credit_limit, principal, start_date, payment_day, card_last4, include_in_net_worth)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		acc.Name, acc.Kind, acc.Type, acc.Currency, acc.OpeningBalance, acc.Archived,
		acc.InterestRate, acc.TermMonths, acc.MaturityDate, acc.Capitalization,
		acc.CreditLimit, acc.Principal, acc.StartDate, acc.PaymentDay, acc.CardLast4, !acc.ExcludedFromNetWorth,
	).Scan(&acc.ID, &acc.CreatedAt)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context, includeArchived bool) ([]entities.Account, error) {
	query := `SELECT ` + accountColumns + ` FROM accounts`
	if !includeArchived {
		query += ` WHERE archived = false`
	}
	query += ` ORDER BY created_at`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []entities.Account
	for rows.Next() {
		a, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("list accounts: %w", scanErr)
		}

		accounts = append(accounts, a)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	return accounts, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Account, error) {
	query := `SELECT ` + accountColumns + ` FROM accounts WHERE id = $1`

	a, err := scanAccount(r.db.Pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	return &a, nil
}

func (r repository) ByCardLast4(ctx context.Context, last4 string) (*entities.Account, error) {
	query := `SELECT ` + accountColumns + ` FROM accounts WHERE card_last4 = $1`

	a, err := scanAccount(r.db.Pool.QueryRow(ctx, query, last4))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account by card: %w", err)
	}

	return &a, nil
}

func (r repository) Update(ctx context.Context, acc *entities.Account) error {
	const query = `
		UPDATE accounts SET
			name = $2, archived = $3, interest_rate = $4, term_months = $5,
			maturity_date = $6, capitalization = $7, credit_limit = $8,
			principal = $9, start_date = $10, payment_day = $11, card_last4 = $12,
			include_in_net_worth = $13
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query,
		acc.ID, acc.Name, acc.Archived, acc.InterestRate, acc.TermMonths,
		acc.MaturityDate, acc.Capitalization, acc.CreditLimit,
		acc.Principal, acc.StartDate, acc.PaymentDay, acc.CardLast4, !acc.ExcludedFromNetWorth,
	)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Delete(ctx context.Context, id uuid.UUID) error {
	var inUse int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE from_account_id = $1 OR to_account_id = $1`, id).
		Scan(&inUse)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if inUse > 0 {
		return ErrInUse
	}

	res, err := r.db.Pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Balances(ctx context.Context) (map[uuid.UUID]int64, error) {
	// One pass: aggregate credited (inflow) and debited (outflow) amounts per
	// account, then apply the asset/liability formula in Go via ledger.Balance.
	const query = `
		SELECT a.id, a.kind, a.opening_balance,
		       COALESCE(inq.s, 0) AS inflow,
		       COALESCE(outq.s, 0) AS outflow
		FROM accounts a
		LEFT JOIN (
			SELECT to_account_id AS aid,
			       SUM(CASE WHEN type = 'transfer' AND to_amount IS NOT NULL
			                THEN to_amount ELSE amount END) AS s
			FROM transactions
			WHERE to_account_id IS NOT NULL
			GROUP BY to_account_id
		) inq ON inq.aid = a.id
		LEFT JOIN (
			SELECT from_account_id AS aid, SUM(amount) AS s
			FROM transactions
			WHERE from_account_id IS NOT NULL
			GROUP BY from_account_id
		) outq ON outq.aid = a.id`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[uuid.UUID]int64)
	for rows.Next() {
		var (
			id               uuid.UUID
			kind             entities.AccountKind
			opening, in, out int64
		)
		if err = rows.Scan(&id, &kind, &opening, &in, &out); err != nil {
			return nil, fmt.Errorf("balances: %w", err)
		}

		balances[id] = ledger.Balance(kind, opening, in, out)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("balances: %w", err)
	}

	return balances, nil
}
