package storage

import (
	"context"
	"errors"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresPoolFromEnv(ctx context.Context) (*pgxpool.Pool, error) {
	connString := os.Getenv("BME_PG_CONNSTRING")
	if connString == "" {
		return nil, errors.New("BME_PG_CONNSTRING is not set")
	}

	return pgxpool.New(ctx, connString)
}


func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		pool: pool,
	}
}