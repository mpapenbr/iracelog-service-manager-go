package postgres

import (
	"context"
	"os"

	"github.com/exaring/otelpgx"
	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

var DBPool *pgxpool.Pool

type PoolConfigOption func(cfg *pgxpool.Config)

func WithTracer(t pgx.QueryTracer) PoolConfigOption {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = t
	}
}

func NewOtlpTracer() pgx.QueryTracer {
	return otelpgx.NewTracer()
}

func NewMyTracer(logger *log.Logger, level log.Level) pgx.QueryTracer {
	return &myQueryTracer{log: logger, level: level}
}

func InitDB() *pgxpool.Pool {
	return InitWithURL(os.Getenv("DATABASE_URL"))
}

func InitWithURL(url string, opts ...PoolConfigOption) *pgxpool.Pool {
	dbConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatal("Unable to parse database config", log.ErrorField(err))
	}

	dbConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}
	for _, opt := range opts {
		opt(dbConfig)
	}

	DBPool, err = pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatal("Unable to create the database pool", log.ErrorField(err))
	}
	if err := DBPool.Ping(context.Background()); err != nil {
		log.Fatal("Unable to get a valid database connection", log.ErrorField(err))
	}
	return DBPool
}

func CloseDB() {
	DBPool.Close()
}

type myQueryTracer struct {
	log   *log.Logger
	level log.Level
}

func (tracer *myQueryTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	// do the logging
	tracer.log.Log(tracer.level, "Executing",
		log.String("sql", data.SQL),
		log.Any("args", data.Args))

	return ctx
}

//nolint:whitespace // can't make the linters happy
func (tracer *myQueryTracer) TraceQueryEnd(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryEndData,
) {
}
