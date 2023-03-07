package server

import (
	"os"
	"os/signal"

	"github.com/spf13/cobra"

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
	cmd.Flags().StringVar(&config.LogLevel,
		"logLevel",
		"info",
		"controls the log level (debug, info, warn, error, fatal)")
	cmd.Flags().StringVar(&config.SQLLogLevel,
		"sqlLogLevel",
		"debug",
		"controls the log level for sql methods")
	cmd.Flags().StringVar(&config.LogFormat,
		"logFormat",
		"json",
		"controls the log output format")
	return cmd
}

func parseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}

//nolint:funlen // by design
func startServer() error {
	var logger *log.Logger
	switch config.LogFormat {
	case "json":
		logger = log.New(
			os.Stderr,
			parseLogLevel(config.LogLevel, log.InfoLevel),
			log.WithCaller(true),
			log.AddCallerSkip(1))
	default:
		logger = log.DevLogger(
			os.Stderr,
			parseLogLevel(config.LogLevel, log.DebugLevel),
			log.WithCaller(true),
			log.AddCallerSkip(1))
	}

	log.ResetDefault(logger)

	log.Debug("Config:",
		log.String("url", config.URL),
		log.String("db", config.DB),
		log.String("realm", config.Realm),
		log.String("password", config.Password),
	)

	log.Info("Starting server")
	pool := postgres.InitWithUrl(
		config.DB,
		postgres.WithTracer(logger, parseLogLevel(config.SQLLogLevel, log.DebugLevel)))

	pm, err := provider.InitProviderEndpoints(pool)
	if err != nil {
		log.Error("server could not be started", log.ErrorField(err))
		return err
	}

	pub, err := public.InitPublicEndpoints(pool)
	if err != nil {
		log.Error("server could not be started", log.ErrorField(err))
		return err
	}

	log.Info("Server started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	v := <-sigChan
	log.Debug("Got signal ", log.Any("signal", v))
	pm.Shutdown()
	pub.Shutdown()

	//nolint:all // keeping by design
	// select {
	// case <-sigChan:
	// 	log.Logger.Debug("Got signal")
	// 	pm.Shutdown()
	// }

	log.Info("Server terminated")
	return nil
}
