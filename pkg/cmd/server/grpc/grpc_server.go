package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // by design
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/analysis/v1/analysisv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/event/v1/eventv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/livedata/v1/livedatav1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/predict/v1/predictv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/provider/v1/providerv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/racestate/v1/racestatev1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/tenant/v1/tenantv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/track/v1/trackv1connect"
	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/pgx-contrib/pgxtrace"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	otlpruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/analysis"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/livedata"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/predict"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/provider"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/state"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/tenant"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/track"
	serverUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/local"
	ownNats "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/nats"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	utilsCache "github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

var appConfig config.Config // holds processed config values

//nolint:funlen // by design
func NewServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "starts the gRPC server",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return startServer(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&config.GrpcServerAddr,
		"grpc-server-addr",
		"a",
		"localhost:8080",
		"gRPC server listen address (insecure)")
	cmd.Flags().StringVar(&config.TLSServerAddr,
		"tls-server-addr",
		"localhost:8081",
		"gRPC server listen address (TLS)")
	cmd.Flags().StringVar(&config.TLSCertFile,
		"tls-cert",
		"",
		"file containing the TLS certificate")
	cmd.Flags().StringVar(&config.TLSKeyFile,
		"tls-key",
		"",
		"file containing the TLS private key")
	cmd.Flags().StringVar(&config.TLSCAFile,
		"tls-ca",
		"",
		"file containing the TLS certificate authority")
	cmd.Flags().StringVar(&config.TraefikCertDomain,
		"traefik-cert-domain",
		"",
		"look fo this domain in the traefik certs")
	cmd.Flags().StringVar(&config.TraefikCerts,
		"traefik-certs",
		"",
		"file containing the certs managed by traefik")

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
	cmd.Flags().BoolVar(&appConfig.SupportTenants,
		"enable-tenants",
		false,
		"enables tenant support")
	cmd.Flags().StringVar(&config.AdminToken,
		"admin-token",
		"",
		"admin token value")
	cmd.Flags().StringVar(&config.ProviderToken,
		"provider-token",
		"",
		"provider token value")
	cmd.Flags().StringVar(&config.StaleDuration,
		"stale-duration",
		"1m",
		"provider is removed if no data was received for this duration")
	cmd.Flags().IntVar(&config.MaxConcurrentStreams,
		"max-concurrent-streams",
		100,
		"max number of concurrent streams per connection")
	cmd.Flags().BoolVar(&config.EnableNats,
		"enable-nats",
		false,
		"use NATS as middleware for live data")
	cmd.Flags().StringVar(&config.NatsURL,
		"nats-url",
		"nats://localhost:4222",
		"url of the NATS server")
	return cmd
}

type grpcServer struct {
	ctx               context.Context
	log               *log.Logger
	pool              *pgxpool.Pool
	mux               *http.ServeMux
	telemetry         *config.Telemetry
	otel              *otelconnect.Interceptor
	eventLookup       *utils.EventLookup
	tlsConfig         *tls.Config
	dataProxy         proxy.DataProxy
	authInterceptor   connect.Interceptor
	configInterceptor connect.Interceptor
	tenantCache       utilsCache.Cache[string, model.Tenant]
}

//nolint:funlen,cyclop // by design
func startServer(ctx context.Context) error {
	srv := grpcServer{
		ctx:       ctx,
		tlsConfig: NewTLSConfigProvider(ctx),
	}
	srv.SetupLogger()
	srv.waitForRequiredServices()
	srv.log.Info("Starting iRaclog backend", log.String("version", version.FullVersion))
	srv.SetupProfiling()
	srv.SetupTelemetry()
	srv.SetupDB()
	srv.SetupCaches()
	srv.SetupFeatures()
	srv.SetupConfigInterceptor()
	srv.SetupAuthInterceptor()
	srv.SetupGrpcServices()
	return srv.Start()
}

func (s *grpcServer) SetupTelemetry() {
	if config.EnableTelemetry {
		s.log.Info("Enabling telemetry")
		err := otlpruntime.Start(otlpruntime.WithMinimumReadMemStatsInterval(time.Second))
		if err != nil {
			s.log.Warn("Could not start runtime metrics", log.ErrorField(err))
		}
	}
}

func (s *grpcServer) SetupProfiling() {
	if config.ProfilingPort > 0 {
		s.log.Info("Starting profiling server on port",
			log.Int("port", config.ProfilingPort))
		go func() {
			//nolint:gosec // by design
			err := http.ListenAndServe(
				fmt.Sprintf("localhost:%d", config.ProfilingPort),
				nil)
			if err != nil {
				s.log.Error("Profiling server stopped", log.ErrorField(err))
			}
		}()
	}
}

func (s *grpcServer) SetupLogger() {
	s.log = log.GetFromContext(s.ctx).Named("grpc")
}

func (s *grpcServer) SetupDB() {
	pgTracer := pgxtrace.CompositeQueryTracer{
		postgres.NewMyTracer(log.Default().Named("sql"), log.DebugLevel),
	}

	if config.EnableTelemetry {
		var err error
		if s.telemetry, err = config.SetupTelemetry(context.Background()); err == nil {
			pgTracer = append(pgTracer, postgres.NewOtlpTracer())
		} else {
			s.log.Warn("Could not setup db telemetry", log.ErrorField(err))
		}
	}

	pgOptions := []postgres.PoolConfigOption{
		postgres.WithTracer(pgTracer),
	}
	s.log.Info("Init database connection")
	s.pool = postgres.InitWithURL(
		config.DB,
		pgOptions...,
	)
}

func (s *grpcServer) SetupFeatures() {
	if appConfig.SupportTenants {
		s.log.Info("Tenant support enabled")
	}
}

func (s *grpcServer) SetupCaches() {
	s.tenantCache = cache.NewTenantCache(s.pool)
}

func (s *grpcServer) SetupAuthInterceptor() {
	s.authInterceptor = auth.NewAuthInterceptor(
		auth.WithAuthToken(config.AdminToken),
		auth.WithTenantCache(s.tenantCache),
	)
}

func (s *grpcServer) SetupConfigInterceptor() {
	s.configInterceptor = serverUtil.NewAppContextInterceptor(&appConfig)
}

func (s *grpcServer) SetupGrpcServices() {
	s.mux = http.NewServeMux()
	s.otel, _ = otelconnect.NewInterceptor(
		otelconnect.WithoutServerPeerAttributes(),
	)
	staleDuration, err := time.ParseDuration(config.StaleDuration)
	if err != nil {
		staleDuration = 1 * time.Minute
	}
	s.log.Debug("init with stale duration", log.Duration("duration", staleDuration))

	// default: use local pubsub
	s.eventLookup = utils.NewEventLookup(utils.WithStaleDuration(staleDuration))
	localInst := local.NewLocalProxy(s.eventLookup)
	s.dataProxy = localInst
	s.eventLookup.SetDeleteEventCB(localInst.DeleteEventCallback)
	// TODO: rename to cluster?
	if config.EnableNats {
		s.log.Debug("Using NATS as middleware for live data")
		nc, err := nats.Connect(config.NatsURL)
		if err != nil {
			s.log.Error("Could not connect to NATS", log.ErrorField(err))
		}
		if inst, err := ownNats.NewNatsProxy(nc,
			ownNats.WithContext(s.ctx),
			ownNats.WithLogger(log.GetFromContext(s.ctx).Named("nats")),
		); err == nil {
			s.dataProxy = inst
			s.eventLookup = utils.NewEventLookup(
				utils.WithStaleDuration(staleDuration),
				utils.WithDeleteEventCB(inst.DeleteEventCallback))
			inst.SetOnUnregisterCB(s.eventLookup.RemoveEvent)
		} else {
			s.log.Warn("Could not setup NATS proxy. Continue with standard mode",
				log.ErrorField(err))
		}
	}
	s.registerEventServer()
	s.registerAnalysisServer()
	s.registerProviderServer()
	s.registerLiveDataServer()
	s.registerStateServer()
	s.registerTrackServer()
	s.registerPredictServer()
	s.registerTenantServer()
	s.registerHealthServer()
	s.registerReflectionServer()
}

//nolint:funlen // by design
func (s *grpcServer) Start() error {
	setupGoRoutinesDump()
	ch := make(chan error, 2)
	if s.tlsConfig != nil {
		//nolint:gocritic // keep it as sample
		// caCert, err := os.ReadFile(config.TLSCertFile)
		// if err != nil {
		// 	s.log.Error("could not read TLS cert file", log.ErrorField(err))
		// }
		// caCertPool := x509.NewCertPool()
		// if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		// 	s.log.Error("could not append cert to pool")
		// }
		// tlsConfig := &tls.Config{
		// 	Certificates: []tls.Certificate{*s.cert},
		// 	MinVersion:   tls.VersionTLS13,
		// 	// keep it as sample
		// 	// ClientCAs:    caCertPool,
		// 	// ClientAuth: tls.NoClientCert,
		// }

		startServerTLS := func() {
			s.log.Info("Starting TLS gRPC server", log.String("addr", config.TLSServerAddr))
			//nolint:gosec // by design
			server := &http.Server{
				Addr:      config.TLSServerAddr,
				TLSConfig: s.tlsConfig,
				Handler: h2c.NewHandler(newCORS().Handler(s.mux), &http2.Server{
					MaxConcurrentStreams: uint32(config.MaxConcurrentStreams),
				}),
			}

			// don't need to pass cert and key here, already done by TLSConfig above
			err := server.ListenAndServeTLS("", "")
			s.log.Error("TLS Server not started", log.ErrorField(err))
			ch <- err
		}
		go startServerTLS()
	}

	startServer := func() {
		s.log.Info("Starting gRPC server", log.String("addr", config.GrpcServerAddr))
		//nolint:gosec // by design
		server := &http.Server{
			Addr: config.GrpcServerAddr,
			Handler: h2c.NewHandler(newCORS().Handler(s.mux), &http2.Server{
				MaxConcurrentStreams: uint32(config.MaxConcurrentStreams),
			}),
		}

		err := server.ListenAndServe()
		s.log.Error("Server not started", log.ErrorField(err))
		ch <- err
	}
	go startServer()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	select {
	case v := <-sigChan:
		s.log.Debug("Got signal", log.Any("signal", v))
	case err := <-ch:
		s.log.Debug("Server terminated", log.ErrorField(err))
	}
	if s.telemetry != nil {
		s.telemetry.Shutdown()
	}
	s.log.Debug("Shutting down pubsub")
	s.dataProxy.Close()

	s.log.Info("Server terminated")
	return nil
}

func (s *grpcServer) waitForRequiredServices() {
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		s.log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}

	wg := sync.WaitGroup{}
	checkTCP := func(addr string) {
		if err = utils.WaitForTCP(addr, timeout); err != nil {
			s.log.Fatal("required services not ready", log.ErrorField(err))
		}
		wg.Done()
	}

	if postgresAddr := utils.ExtractFromDBURL(config.DB); postgresAddr != "" {
		wg.Add(1)
		go checkTCP(postgresAddr)
	}

	// TODO: check for NATS if enabled

	s.log.Debug("Waiting for connection checks to return")
	wg.Wait()
	s.log.Debug("Required services are available")
}

func (s *grpcServer) registerHealthServer() {
	checker := grpchealth.NewStaticChecker()
	s.mux.Handle(grpchealth.NewHandler(checker))
}

func (s *grpcServer) registerReflectionServer() {
	checker := grpcreflect.NewStaticReflector(
		analysisv1connect.AnalysisServiceName,
		eventv1connect.EventServiceName,
		livedatav1connect.LiveDataServiceName,
		predictv1connect.PredictServiceName,
		providerv1connect.ProviderServiceName,
		racestatev1connect.RaceStateServiceName,
		trackv1connect.TrackServiceName,
		tenantv1connect.TenantServiceName)
	s.mux.Handle(grpcreflect.NewHandlerV1(checker))
	s.mux.Handle(grpcreflect.NewHandlerV1Alpha(checker))
}

func (s *grpcServer) registerEventServer() {
	eventService := event.NewServer(
		event.WithPool(s.pool),
		event.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := eventv1connect.NewEventServiceHandler(
		eventService,
		connect.WithInterceptors(s.otel, s.configInterceptor, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerAnalysisServer() {
	analysisService := analysis.NewServer(
		analysis.WithPool(s.pool),
		analysis.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := analysisv1connect.NewAnalysisServiceHandler(
		analysisService,
		connect.WithInterceptors(s.otel, s.configInterceptor, s.authInterceptor),
	)

	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerProviderServer() {
	providerService := provider.NewServer(
		provider.WithPersistence(s.pool),
		provider.WithEventLookup(s.eventLookup),
		provider.WithDataProxy(s.dataProxy),
		provider.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := providerv1connect.NewProviderServiceHandler(
		providerService,
		connect.WithInterceptors(s.otel, s.configInterceptor, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerStateServer() {
	stateService := state.NewServer(
		state.WithPool(s.pool),
		state.WithEventLookup(s.eventLookup), // to be removed?
		state.WithDataProxy(s.dataProxy),
		state.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := racestatev1connect.NewRaceStateServiceHandler(
		stateService,
		connect.WithInterceptors(s.otel, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerLiveDataServer() {
	liveDataService := livedata.NewServer(
		livedata.WithEventLookup(s.eventLookup), // to be removed?
		livedata.WithDataProxy(s.dataProxy),
	)
	path, handler := livedatav1connect.NewLiveDataServiceHandler(
		liveDataService,
		connect.WithInterceptors(s.otel),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerTrackServer() {
	trackService := track.NewServer(
		track.WithPool(s.pool),
		track.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := trackv1connect.NewTrackServiceHandler(
		trackService,
		connect.WithInterceptors(s.otel, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerTenantServer() {
	tenantService := tenant.NewServer(
		tenant.WithPool(s.pool),
		tenant.WithTenantCache(s.tenantCache),
		tenant.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := tenantv1connect.NewTenantServiceHandler(
		tenantService,
		connect.WithInterceptors(s.otel, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerPredictServer() {
	predictService := predict.NewServer(
		predict.WithPool(s.pool),
		predict.WithEventLookup(s.eventLookup), // to be removed?
	)
	path, handler := predictv1connect.NewPredictServiceHandler(
		predictService,
		connect.WithInterceptors(s.otel),
	)
	s.mux.Handle(path, handler)
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
