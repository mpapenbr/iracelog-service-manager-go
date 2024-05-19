//nolint:errcheck,dupl // testsetup
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
func SetupTestDbDeprecated() *pgxpool.Pool {
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

	if err = migrate.MigrateDbDeprecated(dbUrl); err != nil {
		log.Fatal(err)
	}

	pool := database.InitWithUrl(dbUrl)
	return pool
}

func ClearEventTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from event_ext")
	pool.Exec(context.Background(), "delete from event")
}

func ClearCarTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from car")
}

func ClearTrackTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from track")
}

func ClearDriverTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from driver")
}

func ClearSpeedmapTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from speedmap")
}

func ClearAnalysisTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from analysis")
}

func ClearWampDataTableDeprecated(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from wampdata")
}

func ClearAllTablesDeprecated(pool *pgxpool.Pool) {
	ClearWampDataTableDeprecated(pool)
	ClearSpeedmapTableDeprecated(pool)
	ClearAnalysisTableDeprecated(pool)
	ClearCarTableDeprecated(pool)
	ClearDriverTableDeprecated(pool)
	ClearTrackTableDeprecated(pool)
	ClearEventTableDeprecated(pool)
}
