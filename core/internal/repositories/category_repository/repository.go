package categoryrepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
)

var (
	ErrNotFound = errors.New("category not found")
	ErrInUse    = errors.New("category has subcategories or transactions")
)

type Repository interface {
	Create(ctx context.Context, cat *entities.Category) error
	List(ctx context.Context, typ *entities.CategoryType, includeArchived bool) ([]entities.Category, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Category, error)
	Update(ctx context.Context, cat *entities.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

const categoryColumns = `id, name, parent_id, type, icon, color, archived, created_at`

func scanCategory(row pgx.Row) (entities.Category, error) {
	var c entities.Category
	err := row.Scan(&c.ID, &c.Name, &c.ParentID, &c.Type, &c.Icon, &c.Color, &c.Archived, &c.CreatedAt)

	return c, err
}

func (r repository) Create(ctx context.Context, cat *entities.Category) error {
	const query = `
		INSERT INTO categories (name, parent_id, type, icon, color)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query, cat.Name, cat.ParentID, cat.Type, cat.Icon, cat.Color).
		Scan(&cat.ID, &cat.CreatedAt)
	if err != nil {
		return fmt.Errorf("create category: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context, typ *entities.CategoryType, includeArchived bool) ([]entities.Category, error) {
	query := `SELECT ` + categoryColumns + ` FROM categories WHERE 1 = 1`
	args := []any{}

	if typ != nil {
		args = append(args, *typ)
		query += fmt.Sprintf(` AND type = $%d`, len(args))
	}
	if !includeArchived {
		query += ` AND archived = false`
	}
	// parents before their children, then by name for a stable tree order
	query += ` ORDER BY parent_id NULLS FIRST, name`

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var categories []entities.Category
	for rows.Next() {
		c, scanErr := scanCategory(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("list categories: %w", scanErr)
		}

		categories = append(categories, c)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	return categories, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Category, error) {
	query := `SELECT ` + categoryColumns + ` FROM categories WHERE id = $1`

	c, err := scanCategory(r.db.Pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}

	return &c, nil
}

func (r repository) Update(ctx context.Context, cat *entities.Category) error {
	const query = `
		UPDATE categories SET name = $2, icon = $3, color = $4, archived = $5
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query, cat.ID, cat.Name, cat.Icon, cat.Color, cat.Archived)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Delete(ctx context.Context, id uuid.UUID) error {
	var inUse int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM categories   WHERE parent_id   = $1) +
			(SELECT count(*) FROM transactions WHERE category_id = $1)`, id).Scan(&inUse)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if inUse > 0 {
		return ErrInUse
	}

	res, err := r.db.Pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
