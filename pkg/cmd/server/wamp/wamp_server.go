package wamp

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // by design
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/pgx-contrib/pgxtrace"
	"github.com/spf13/cobra"
	otlpruntime "go.opentelemetry.io/contrib/instrumentation/runtime"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/admin"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/provider"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/public"
	eputils "github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

var appConfig config.Config // holds processed config values

//nolint:funlen // by design
func NewServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wamp",
		Short: "starts the WAMP server",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			appConfig = config.Config{}
			return nil
		},
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
	cmd.Flags().BoolVar(&config.EnableTelemetry,
		"enable-telemetry",
		false,
		"enables telemetry")
	cmd.Flags().StringVar(&config.TelemetryEndpoint,
		"telemetry-endpoint",
		"localhost:4317",
		"Endpoint that receives open telemetry data")
	cmd.Flags().IntVar(&config.ProfilingPort,
		"profiling-port",
		0,
		"port to use for providing profiling data")
	cmd.Flags().BoolVar(&appConfig.PrintMessage,
		"print-message",
		false,
		"if true and log level is debug, the message payload will be printed")
	return cmd
}

func parseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}

//nolint:funlen,cyclop // by design
func startServer() error {
	var logger *log.Logger
	var sqlLogger *log.Logger
	var telemetry *config.Telemetry
	switch config.LogFormat {
	case "json":
		logger = log.New(
			os.Stderr,
			parseLogLevel(config.LogLevel, log.InfoLevel),
			log.WithCaller(true),
			log.AddCallerSkip(1))
		sqlLogger = log.New(
			os.Stderr,
			parseLogLevel(config.SQLLogLevel, log.InfoLevel),
			log.WithCaller(true),
			log.AddCallerSkip(1))

	default:
		logger = log.DevLogger(
			os.Stderr,
			parseLogLevel(config.LogLevel, log.DebugLevel),
			log.WithCaller(true),
			log.AddCallerSkip(1))

		sqlLogger = log.DevLogger(
			os.Stderr,
			parseLogLevel(config.SQLLogLevel, log.InfoLevel),
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

	if config.ProfilingPort > 0 {
		log.Info("Starting profiling server on port", log.Int("port", config.ProfilingPort))
		go func() {
			//nolint:gosec // by design
			err := http.ListenAndServe(
				fmt.Sprintf("localhost:%d", config.ProfilingPort),
				nil)
			if err != nil {
				log.Error("Profiling server stopped", log.ErrorField(err))
			}
		}()
	}

	waitForRequiredServices()

	pgTracer := pgxtrace.CompositeQueryTracer{
		postgres.NewMyTracer(sqlLogger, log.DebugLevel),
	}

	if config.EnableTelemetry {
		logger.Info("Enabling telemetry")
		var err error
		if telemetry, err = config.SetupTelemetry(context.Background()); err == nil {
			pgTracer = append(pgTracer, postgres.NewOtlpTracer())
		} else {
			logger.Warn("Could not setup telemetry", log.ErrorField(err))
		}
		err = otlpruntime.Start(otlpruntime.WithMinimumReadMemStatsInterval(time.Second))
		if err != nil {
			logger.Warn("Could not start runtime metrics", log.ErrorField(err))
		}
	}

	pgOptions := []postgres.PoolConfigOption{
		postgres.WithTracer(pgTracer),
	}
	log.Info("Starting server")
	pool := postgres.InitWithUrl(
		config.DB,
		pgOptions...,
	)

	pm, err := provider.NewProviderManager(
		provider.WithPersistence(pool),
		provider.WithConfig(appConfig),
		provider.WithWampClient(getWampClient(logger)),
	)
	if err != nil {
		log.Error("server could not be started", log.ErrorField(err))
		return err
	}

	pub, err := public.NewPublicManager(
		public.WithPersistence(pool),
		public.WithConfig(appConfig),
		public.WithWampClient(getWampClient(logger)),
		public.WithProviderLookup(pm.ProviderLookupFunc),
	)
	if err != nil {
		log.Error("server could not be started", log.ErrorField(err))
		return err
	}

	adm, err := admin.NewAdminManager(
		admin.WithPersistence(pool),
		admin.WithWampClient(getWampClient(logger)),
	)
	if err != nil {
		log.Error("server could not be started", log.ErrorField(err))
		return err
	}

	log.Info("Server started")
	setupGoRoutinesDump()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	v := <-sigChan
	log.Debug("Got signal ", log.Any("signal", v))
	pm.Shutdown()
	pub.Shutdown()
	adm.Shutdown()
	if telemetry != nil {
		telemetry.Shutdown()
	}

	//nolint:all // keeping by design
	// select {
	// case <-sigChan:
	// 	log.Logger.Debug("Got signal")
	// 	pm.Shutdown()
	// }

	log.Info("Server terminated")
	return nil
}

func getWampClient(logger *log.Logger) *client.Client {
	ret, err := eputils.NewClient(eputils.WithClientLogging(logger))
	if err != nil {
		log.Fatal("Could not connect wamp client", log.ErrorField(err))
	}
	return ret
}

func setupGoRoutinesDump() {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGQUIT)
		buf := make([]byte, 1<<20)
		for {
			<-sigs
			stacklen := runtime.Stack(buf, true)
			fmt.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n",
				buf[:stacklen])
		}
	}()
}

func waitForRequiredServices() {
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}

	wg := sync.WaitGroup{}
	checkTcp := func(addr string) {
		if err = utils.WaitForTCP(addr, timeout); err != nil {
			log.Fatal("required services not ready", log.ErrorField(err))
		}
		wg.Done()
	}
	checkHttp := func(url string) {
		if err = utils.WaitForHttpResponse(url, timeout); err != nil {
			log.Fatal("required services not ready", log.ErrorField(err))
		}
		wg.Done()
	}
	if wsAddr, proto := utils.ExtractFromWebsocketUrl(config.URL); wsAddr != "" {
		wg.Add(1)
		go checkTcp(wsAddr)
		wg.Add(1)
		var url string
		if proto == "wss" {
			url = fmt.Sprintf("https://%s/info", wsAddr)
		} else {
			url = fmt.Sprintf("http://%s/info", wsAddr)
		}
		go checkHttp(url)
	}
	if postgresAddr := utils.ExtractFromDBUrl(config.DB); postgresAddr != "" {
		wg.Add(1)
		go checkTcp(postgresAddr)
	}
	log.Debug("Waiting for connection checks to return")
	wg.Wait()
	log.Debug("Required services are available")
}
