package migrate

import (
	"embed"
	"errors"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var migrations embed.FS

//go:embed migrations-deprecated
var migrationsDeprecated embed.FS

func MigrateDb(dbURI string) error {
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", source,
		strings.Replace(dbURI, "postgresql://", "pgx://", 1))
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func MigrateDbDeprecated(dbURI string) error {
	source, err := iofs.New(migrationsDeprecated, "migrations-deprecated")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", source,
		strings.Replace(dbURI, "postgresql://", "pgx://", 1))
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
