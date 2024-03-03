package grpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // by design
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	otlpruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"google.golang.org/grpc"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"

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
	cmd.Flags().IntVarP(&config.GrpcPort,
		"grpc-port",
		"p",
		8084,
		"gRPC port to listen on")

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
	// TODO: remove dummy
	if pool == nil {
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", config.GrpcPort))
	if err != nil {
		log.Fatal("failed to listen: %v", log.ErrorField(err))
	}
	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	// pb.RegisterRouteGuideServer(grpcServer, newServer())
	grpcServer.Serve(lis)

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
