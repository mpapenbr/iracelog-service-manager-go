package grpc

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

	"buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/event/v1/eventv1connect"
	"buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/provider/v1/providerv1connect"
	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	otlpruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/provider"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

var appConfig config.Config // holds processed config values

//nolint:funlen // by design
func NewServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "starts the gRPC server",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			appConfig = config.Config{}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return startServer()
		},
	}
	cmd.Flags().StringVarP(&config.GrpcServerAddr,
		"grpc-server-addr",
		"a",
		"localhost:8080",
		"gRPC server listen address")

	cmd.Flags().StringVar(&config.LogLevel,
		"log-level",
		"info",
		"controls the log level (debug, info, warn, error, fatal)")
	cmd.Flags().StringVar(&config.SQLLogLevel,
		"sql-log-level",
		"debug",
		"controls the log level for sql methods")
	cmd.Flags().StringVar(&config.LogFormat,
		"log-format",
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
	cmd.Flags().StringVar(&config.AdminToken,
		"admin-token",
		"",
		"admin token value")
	cmd.Flags().StringVar(&config.ProviderToken,
		"provider-token",
		"",
		"provider token value")
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

	pgTraceOption := postgres.WithTracer(sqlLogger, log.DebugLevel)
	if config.EnableTelemetry {
		log.Info("Enabling telemetry")
		var err error
		if telemetry, err = config.SetupTelemetry(context.Background()); err == nil {
			pgTraceOption = postgres.WithOtlpTracer()
		} else {
			log.Warn("Could not setup telemetry", log.ErrorField(err))
		}
		err = otlpruntime.Start(otlpruntime.WithMinimumReadMemStatsInterval(time.Second))
		if err != nil {
			log.Warn("Could not start runtime metrics", log.ErrorField(err))
		}
	}

	log.Info("Starting server")
	pool := postgres.InitWithUrl(
		config.DB,
		pgTraceOption,
	)

	mux := registerGrpcServices(pool)

	startServer := func() error {
		log.Info("Starting gRPC server", log.String("addr", config.GrpcServerAddr))
		//nolint:gosec // by design
		server := &http.Server{
			Addr:    config.GrpcServerAddr,
			Handler: h2c.NewHandler(newCORS().Handler(mux), &http2.Server{}),
		}
		return server.ListenAndServe()
	}
	if err := startServer(); err != nil {
		log.Error("server could not be started", log.ErrorField(err))
		return err
	}
	log.Info("Server started")
	setupGoRoutinesDump()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	v := <-sigChan
	log.Debug("Got signal ", log.Any("signal", v))
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

func registerGrpcServices(pool *pgxpool.Pool) *http.ServeMux {
	mux := http.NewServeMux()
	myOtel, _ := otelconnect.NewInterceptor()
	registerEventServer(mux, pool, myOtel)
	registerProviderServer(mux, pool, myOtel)
	return mux
}

//nolint:whitespace // can't make both editor and linter happy
func registerEventServer(
	mux *http.ServeMux, pool *pgxpool.Pool, otel connect.Interceptor,
) {
	eventService := event.NewServer(event.WithPool(pool))
	path, handler := eventv1connect.NewEventServiceHandler(
		eventService,
		connect.WithInterceptors(otel),
	)
	mux.Handle(path, handler)
}

//nolint:whitespace // can't make both editor and linter happy
func registerProviderServer(
	mux *http.ServeMux, pool *pgxpool.Pool, otel connect.Interceptor,
) {
	providerService := provider.NewServer(provider.WithPool(pool),
		provider.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := providerv1connect.NewProviderServiceHandler(
		providerService,
		connect.WithInterceptors(otel,
			auth.NewAuthInterceptor(auth.WithAuthToken(config.AdminToken),
				auth.WithProviderToken(config.ProviderToken))),
	)
	mux.Handle(path, handler)
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

	if postgresAddr := utils.ExtractFromDBUrl(config.DB); postgresAddr != "" {
		wg.Add(1)
		go checkTcp(postgresAddr)
	}
	log.Debug("Waiting for connection checks to return")
	wg.Wait()
	log.Debug("Required services are available")
}

func newCORS() *cors.Cors {
	// To let web developers play with the demo service from browsers, we need a
	// very permissive CORS setup.
	return cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(origin string) bool {
			// Allow all origins, which effectively disables CORS.
			return true
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			// Content-Type is in the default safelist.
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
		},
		// Let browsers cache CORS information for longer, which reduces the number
		// of preflight requests. Any changes to ExposedHeaders won't take effect
		// until the cached data expires. FF caps this value at 24h, and modern
		// Chrome caps it at 2h.
		MaxAge: int(2 * time.Hour / time.Second),
	})
}
