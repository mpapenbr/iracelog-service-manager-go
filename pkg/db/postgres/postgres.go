package postgres

import (
	"context"
	"log"
	"os"

	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var DbPool *pgxpool.Pool

type PoolConfigOption func(cfg *pgxpool.Config)

func WithTracer(logger *zap.SugaredLogger) PoolConfigOption {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = &myQueryTracer{log: logger}
	}
}

func InitDB() *pgxpool.Pool {
	return InitWithUrl(os.Getenv("DATABASE_URL"))
}

func InitWithUrl(url string, opts ...PoolConfigOption) *pgxpool.Pool {
	dbConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatalf("Unable to parse database config %v\n", err)
	}

	//nolint:all  dbConfig.ConnConfig.Tracer = &myQueryTracer{log: mylog.Logger.Sugar()}
	dbConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}
	for _, opt := range opts {
		opt(dbConfig)
	}

	DbPool, err = pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatalf("Unable to connect to database %v\n", err)
	}
	return DbPool
}

func CloseDb() {
	DbPool.Close()
}

type myQueryTracer struct {
	log *zap.SugaredLogger
	// level zapcore.Level
}

func (tracer *myQueryTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	tracer.log.Infow("Executing command", "sql", data.SQL, "args", data.Args)

	return ctx
}

func (tracer *myQueryTracer) TraceQueryEnd(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryEndData) {

}
