package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//nolint:lll // ok for interface
type Querier interface {
	// Begin(ctx context.Context) (pgx.Tx, error)
	// BeginFunc(ctx context.Context, f func(pgx.Tx) error) (err error)
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

var (
	_ Querier = (*pgx.Conn)(nil)
	_ Querier = (*pgxpool.Pool)(nil)
	_ Querier = pgx.Tx(nil)
)
