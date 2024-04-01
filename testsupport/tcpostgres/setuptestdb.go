//nolint:errcheck // testsetup
package tcpostgres

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/migrate"
	database "github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
)

// create a pg connection pool for the iracelog testdatabase
func SetupTestDb() *pgxpool.Pool {
	ctx := context.Background()
	port, err := nat.NewPort("tcp", "5432")
	if err != nil {
		log.Fatal(err)
	}
	container, err := SetupPostgres(ctx,
		WithPort(port.Port()),
		WithInitialDatabase("postgres", "password", "postgres"),
		WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
		WithName("iracelog-service-manager-test"),
	)
	if err != nil {
		log.Fatal(err)
	}
	//nolint:gocritic // debug
	// fmt.Printf("%+v\n", container)
	containerPort, _ := container.MappedPort(ctx, port)
	host, _ := container.Host(ctx)
	dbUrl := fmt.Sprintf("postgresql://postgres:password@%s:%s/postgres",
		host, containerPort.Port())

	if err = migrate.MigrateDb(dbUrl); err != nil {
		log.Fatal(err)
	}

	pool := database.InitWithUrl(dbUrl)
	return pool
}

func ClearEventTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from event_ext")
	pool.Exec(context.Background(), "delete from event")
}

func ClearCarTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from car")
}

func ClearTrackTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from track")
}

func ClearDriverTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from driver")
}

func ClearSpeedmapTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from speedmap")
}

func ClearAnalysisTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from analysis")
}

func ClearWampDataTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from wampdata")
}

func ClearAllTables(pool *pgxpool.Pool) {
	ClearWampDataTable(pool)
	ClearSpeedmapTable(pool)
	ClearAnalysisTable(pool)
	ClearCarTable(pool)
	ClearDriverTable(pool)
	ClearTrackTable(pool)
	ClearEventTable(pool)
}
