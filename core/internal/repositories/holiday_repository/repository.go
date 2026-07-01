package holidayrepository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
)

type Repository interface {
	List(ctx context.Context) ([]entities.Holiday, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

func (r repository) List(ctx context.Context) ([]entities.Holiday, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT day, name FROM holidays ORDER BY day`)
	if err != nil {
		return nil, fmt.Errorf("list holidays: %w", err)
	}

	holidays, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Holiday])
	if err != nil {
		return nil, fmt.Errorf("list holidays: %w", err)
	}

	return holidays, nil
}
