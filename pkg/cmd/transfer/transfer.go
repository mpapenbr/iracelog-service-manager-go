package transfer

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
)

func NewTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "commands to transfer data to new database",
	}

	cmd.PersistentFlags().StringVar(&sourceDbUrl,
		"source-db-url",
		"postgresql://DB_USERNAME:DB_USER_PASSWORD@DB_HOST:5432/iracelog",
		"database connection string for the source database")

	cmd.AddCommand(NewTransferTracksCmd())
	cmd.AddCommand(NewTransferEventCmd())
	return cmd
}

var (
	logLevel    string
	sourceDbUrl string
	poolSource  *pgxpool.Pool
	poolDest    *pgxpool.Pool
)

func parseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}

func setupDatabases() {
	dbDestUrl := prepareDbUrl(config.DB)
	dbSourceUrl := prepareDbUrl(sourceDbUrl)
	log.Info("Using source ", log.String("url", dbSourceUrl))
	log.Info("Using destination ", log.String("url", dbDestUrl))

	poolDest = postgres.InitWithUrl(dbDestUrl)
	poolSource = postgres.InitWithUrl(dbSourceUrl)
}

func prepareDbUrl(url string) string {
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
