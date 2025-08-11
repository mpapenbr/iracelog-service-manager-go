package migrate

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "performs database migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return startMigration()
		},
	}

	cmd.Flags().StringVarP(&config.MigrationSourceURL,
		"migrationSourceUrl",
		"m",
		"file:///migrations",
		"url to migration files")

	return cmd
}

func startMigration() error {
	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}
	postgresAddr := utils.ExtractFromDBURL(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		log.Fatal("database  not ready", log.ErrorField(err))
	}

	log.Info("Using migrations files at", log.String("source", config.MigrationSourceURL))
	dbURL := prepareURLForDB(config.DB)
	log.Info("Using dbUrl", log.String("url", dbURL))

	m, err := migrate.New(config.MigrationSourceURL, dbURL)
	if err != nil {
		log.Fatal("Could not create migration", log.ErrorField(err))
	}
	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		log.Info("No Migration required")
		return nil
	}
	return err
}

func prepareURLForDB(url string) string {
	options := "sslmode=disable"
	if strings.Contains(url, options) {
		return url
	}
	if strings.Contains(url, "?") {
		return fmt.Sprintf("%s&%s", url, options)
	} else {
		return fmt.Sprintf("%s?%s", url, options)
	}
}
