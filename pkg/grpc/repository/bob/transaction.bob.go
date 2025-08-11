package bob

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type bobTransaction struct {
	db *bob.DB
}

var _ api.TransactionManager = (*bobTransaction)(nil)

// @deprecated
// NewBobTransaction creates a new transaction manager using the provided bob.DB.
func NewBobTransaction(db *bob.DB) api.TransactionManager {
	return &bobTransaction{
		db: db,
	}
}

func NewTransactionManager(db bob.DB) api.TransactionManager {
	return &bobTransaction{
		db: &db,
	}
}

func NewBobTransactionFromPool(pool *pgxpool.Pool) api.TransactionManager {
	x := bob.NewDB(stdlib.OpenDBFromPool(pool))
	return &bobTransaction{
		db: &x,
	}
}

// the contract with the repositories is:
// we put the current executor into the context, the repository should first look
// in the context for an executor and then use it to execute queries
//
//nolint:whitespace //editor/linter issue
func (b *bobTransaction) RunInTx(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	return b.db.RunInTx(ctx, nil, func(ctx context.Context, e bob.Executor) error {
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = bobCtx.NewContext(ctx, e)
		return fn(ctx)
	})
}
