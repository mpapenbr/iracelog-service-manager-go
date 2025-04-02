package testdb

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
)

func InitTestDB() *pgxpool.Pool {
	var pool *pgxpool.Pool

	if os.Getenv("TESTDB_URL") != "" {
		pool = tcpg.SetupExternalTestDB()
	} else {
		pool = tcpg.SetupTestDB()
	}
	if err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		tcpg.ClearAllTables(pool)
		return nil
	}); err != nil {
		log.Fatalf("initTestDb: %v\n", err)
	}
	return pool
}
