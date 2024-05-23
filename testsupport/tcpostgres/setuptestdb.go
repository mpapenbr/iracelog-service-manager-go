//nolint:errcheck,dupl // testsetup
package tcpostgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
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
	//nolint fmt.Printf("%+v\n", container)
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

// create a pg connection pool for the local iracelog testdatabase
func SetupExternalTestDb() *pgxpool.Pool {
	var err error

	dbUrl := os.Getenv("TESTDB_URL")
	if err = migrate.MigrateDb(dbUrl); err != nil {
		log.Fatal(err)
	}

	pool := database.InitWithUrl(dbUrl)
	return pool
}

func ClearEventTable(pool *pgxpool.Pool) {
	if _, err := pool.Exec(context.Background(),
		"delete from event"); err != nil {
		log.Fatalf("ClearEventTable: %v\n", err)
	}
}

func ClearCarTable(pool *pgxpool.Pool) {
	if _, err := pool.Exec(context.Background(),
		"delete from car_state_proto"); err != nil {
		log.Fatalf("ClearCarStateProtoTable: %v\n", err)
	}
}

func ClearTrackTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from track")
}

func ClearDriverTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from driver")
}

func ClearSpeedmapTable(pool *pgxpool.Pool) {
	if _, err := pool.Exec(context.Background(),
		"delete from speedmap_proto"); err != nil {
		log.Fatalf("ClearSpeedmapProtoTable: %v\n", err)
	}
}

func ClearAnalysisTable(pool *pgxpool.Pool) {
	pool.Exec(context.Background(), "delete from analysis")
}

func ClearStateDataTable(pool *pgxpool.Pool) {
	if _, err := pool.Exec(context.Background(),
		"delete from race_state_proto"); err != nil {
		log.Fatalf("ClearStateDataTable: %v\n", err)
	}
}

func clearTables(pool *pgxpool.Pool, tables []string) error {
	err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		for _, table := range tables {
			if _, err := tx.Exec(context.Background(), "delete from "+table); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

//nolint:gocritic // wip
func ClearAllTables(pool *pgxpool.Pool) {
	tables := []string{
		"race_state_proto",
		"msg_state_proto",
		"car_state_proto",
		"speedmap_proto",
		"analysis_proto",
		"c_car_driver",
		"c_car_team",
		"c_car_entry",
		"c_car",
		"c_car_class",
		"rs_info",
		"event_ext",
		"event",
		"track",
	}
	err := clearTables(pool, tables)
	if err != nil {
		log.Fatalf("ClearAllTables: %v\n", err)
	}
}
