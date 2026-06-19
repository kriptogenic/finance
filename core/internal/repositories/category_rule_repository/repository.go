package categoryrulerepository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
)

var ErrNotFound = errors.New("category rule not found")

type Repository interface {
	Create(ctx context.Context, r *entities.CategoryRule) error
	// List returns routing rules only (those with a category).
	List(ctx context.Context) ([]entities.CategoryRule, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.CategoryRule, error)
	Update(ctx context.Context, r *entities.CategoryRule) error
	Delete(ctx context.Context, id uuid.UUID) error
	// ListBlocked returns "blocks" — rules with no category (merchants never
	// offered as a rule).
	ListBlocked(ctx context.Context) ([]entities.CategoryRule, error)
	// AddBlock records a blocked merchant (lower-cased); a no-op if already blocked.
	AddBlock(ctx context.Context, merchant string) (entities.CategoryRule, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

const columns = `id, pattern, category_id, created_at`

func scanRule(row pgx.Row) (entities.CategoryRule, error) {
	var r entities.CategoryRule
	err := row.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.CreatedAt)

	return r, err
}

func (repo repository) Create(ctx context.Context, r *entities.CategoryRule) error {
	const query = `
		INSERT INTO category_rules (pattern, category_id)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := repo.db.Pool.QueryRow(ctx, query, r.Pattern, r.CategoryID).Scan(&r.ID, &r.CreatedAt)
	if err != nil {
		return fmt.Errorf("create category rule: %w", err)
	}

	return nil
}

func (repo repository) List(ctx context.Context) ([]entities.CategoryRule, error) {
	// Routing rules only (NULL category_id is a block). Longest pattern first
	// mirrors the ingest match precedence (ResolveForIngest).
	return repo.query(ctx, `SELECT `+columns+` FROM category_rules
		WHERE category_id IS NOT NULL ORDER BY length(pattern) DESC, pattern`)
}

func (repo repository) ListBlocked(ctx context.Context) ([]entities.CategoryRule, error) {
	return repo.query(ctx, `SELECT `+columns+` FROM category_rules
		WHERE category_id IS NULL ORDER BY created_at DESC`)
}

func (repo repository) query(ctx context.Context, sql string) ([]entities.CategoryRule, error) {
	rows, err := repo.db.Pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list category rules: %w", err)
	}
	defer rows.Close()

	var rules []entities.CategoryRule
	for rows.Next() {
		rule, scanErr := scanRule(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("list category rules: %w", scanErr)
		}

		rules = append(rules, rule)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list category rules: %w", err)
	}

	return rules, nil
}

func (repo repository) AddBlock(ctx context.Context, merchant string) (entities.CategoryRule, error) {
	pattern := strings.ToLower(strings.TrimSpace(merchant))

	var r entities.CategoryRule
	err := repo.db.Pool.QueryRow(ctx, `
		INSERT INTO category_rules (pattern, category_id)
		VALUES ($1, NULL)
		ON CONFLICT (pattern) WHERE category_id IS NULL
		DO UPDATE SET pattern = EXCLUDED.pattern
		RETURNING `+columns, pattern).Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.CreatedAt)
	if err != nil {
		return entities.CategoryRule{}, fmt.Errorf("add category rule block: %w", err)
	}

	return r, nil
}

func (repo repository) Get(ctx context.Context, id uuid.UUID) (*entities.CategoryRule, error) {
	rule, err := scanRule(repo.db.Pool.QueryRow(ctx, `SELECT `+columns+` FROM category_rules WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get category rule: %w", err)
	}

	return &rule, nil
}

func (repo repository) Update(ctx context.Context, r *entities.CategoryRule) error {
	const query = `UPDATE category_rules SET pattern = $2, category_id = $3 WHERE id = $1`

	res, err := repo.db.Pool.Exec(ctx, query, r.ID, r.Pattern, r.CategoryID)
	if err != nil {
		return fmt.Errorf("update category rule: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (repo repository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := repo.db.Pool.Exec(ctx, `DELETE FROM category_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete category rule: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
