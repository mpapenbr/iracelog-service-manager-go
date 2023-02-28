package server

import (
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/provider"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/public"
)

func NewServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "starts the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return startServer()
		},
	}
	cmd.Flags().StringVarP(&config.Password,
		"password",
		"p",
		"",
		"Password for backend user in realm")
	return cmd
}

func startServer() error {
	log.InitDevelopmentLogger()

	log.Logger.Debug("Config:",
		zap.String("url", config.URL),
		zap.String("db", config.DB),
		zap.String("realm", config.Realm),
		zap.String("password", config.Password),
	)

	log.Logger.Info("Starting server")
	pool := postgres.InitWithUrl(config.DB, postgres.WithTracer(log.Logger.Sugar()))

	pm, err := provider.InitProviderEndpoints(pool)
	if err != nil {
		log.Logger.Error("server could not be started", zap.Error(err))
		return err
	}

	pub, err := public.InitPublicEndpoints(pool)
	if err != nil {
		log.Logger.Error("server could not be started", zap.Error(err))
		return err
	}

	log.Logger.Info("Server started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	v := <-sigChan
	log.Logger.Debug("Got signal ", zap.Any("signal", v))
	pm.Shutdown()
	pub.Shutdown()

	//nolint:all // keeping by design
	// select {
	// case <-sigChan:
	// 	log.Logger.Debug("Got signal")
	// 	pm.Shutdown()
	// }

	log.Logger.Info("Server terminated")
	return nil
}
