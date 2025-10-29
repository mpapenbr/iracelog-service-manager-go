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
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/auth/v1/authv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/event/v1/eventv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/livedata/v1/livedatav1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/predict/v1/predictv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/provider/v1/providerv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/racestate/v1/racestatev1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/settings/v1/settingsv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/tenant/v1/tenantv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/track/v1/trackv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/user/v1/userv1connect"
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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	authImpl "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth/impl"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc"
	oidcFactory "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc/factory"
	oidcNatsImpl "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc/impl/nats"
	oidcSimplImpl "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc/impl/simple"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	reposApi "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobRepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/analysis"
	authServer "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/livedata"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/predict"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/provider"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/settings"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/state"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/tenant"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/user"
	serverUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/local"
	ownNats "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/nats"
	eventService "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/service/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
	sessionFactory "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session/factory"
	sessionImpl "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session/impl"
	oauth2SessConfig "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session/impl/oauth2"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	utilsCache "github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

var appConfig config.Config // holds processed config values

//nolint:funlen,lll // by design
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
		"Use NATS as middleware for live data")
	cmd.Flags().StringVar(&config.NatsURL,
		"nats-url",
		"nats://localhost:4222",
		"URL of the NATS server")
	cmd.Flags().BoolVar(&appConfig.SupportIDP,
		"enable-idp",
		false,
		"use IDP to authenticate users")
	cmd.Flags().StringVar(&config.IDPClientID,
		"idp-client-id",
		"",
		"IDP client ID")
	cmd.Flags().StringVar(&config.IDPClientSecret,
		"idp-client-secret",
		"",
		"IDP client secret")
	cmd.Flags().StringVar(&config.IDPIssuerURL,
		"idp-issuer-url",
		"",
		"Issuer URL is used to resolve entry points (for example: http://localhost:8080/realms/demo)")
	cmd.Flags().StringVar(&config.IDPCallbackURL,
		"idp-callback-url",
		"",
		"Callback URL is used to redirect users after authentication")
	cmd.Flags().DurationVar(&config.IDPTokenRefreshThreshold,
		"idp-token-refresh-threshold",
		10*time.Second,
		"refresh a token this time before it expires")
	cmd.Flags().DurationVar(&config.SessionTimeout,
		"session-timeout",
		5*time.Minute,
		"Idle time after which a session is removed")

	return cmd
}

type grpcServer struct {
	ctx                   context.Context
	log                   *log.Logger
	pool                  *pgxpool.Pool
	mux                   *http.ServeMux
	telemetry             *config.Telemetry
	otel                  *otelconnect.Interceptor
	eventLookup           *utils.EventLookup
	tlsConfig             *tls.Config
	dataProxy             proxy.DataProxy
	oidcParam             *oidc.OIDCParam
	authInterceptor       connect.Interceptor
	sessionInterceptor    connect.Interceptor
	configInterceptor     connect.Interceptor
	traceIDInterceptor    connect.Interceptor
	tenantCache           utilsCache.Cache[string, model.Tenant]
	sessionStore          session.SessionStore
	pendingAuthStateCache oidc.PendingAuthStateCache
	repos                 reposApi.Repositories
	txManager             reposApi.TransactionManager
	tracer                trace.Tracer
}

//nolint:funlen,cyclop // by design
func startServer(ctx context.Context) error {
	srv := grpcServer{
		ctx:       ctx,
		tlsConfig: NewTLSConfigProvider(ctx),
	}
	srv.tracer = otel.Tracer("ism")
	srv.SetupLogger()
	srv.waitForRequiredServices()
	srv.log.Info("Starting iRaclog backend", log.String("version", version.FullVersion))
	srv.SetupProfiling()
	srv.SetupTelemetry()
	srv.SetupDB()
	srv.SetupFeatures()
	srv.SetupIDPParam()
	srv.SetupConfigInterceptor()
	srv.SetupTraceIDInterceptor()
	srv.SetupTransactionManager()
	srv.SetupRepositories()
	srv.SetupCaches()
	srv.SetupSessionStore()
	srv.SetupPendingAuthStateCache()
	srv.SetupAuthInterceptor() // needs to be after repos + caches
	srv.SetupSessionInterceptor()
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
	if appConfig.SupportIDP {
		s.log.Info("Identity provider support enabled")
	}
}

func (s *grpcServer) SetupCaches() {
	s.tenantCache = cache.NewTenantCache(s.repos.Tenant())
}

func (s *grpcServer) SetupIDPParam() {
	if !appConfig.SupportIDP {
		s.log.Info("IDP based authentication disabled")
		return
	}

	s.oidcParam = &oidc.OIDCParam{
		IssuerURL:    config.IDPIssuerURL,
		ClientID:     config.IDPClientID,
		ClientSecret: config.IDPClientSecret,
		CallbackURL:  config.IDPCallbackURL,
	}
}

//nolint:lll // readability
func (s *grpcServer) SetupSessionStore() {
	var err error
	extraStoreOptions := []oauth2SessConfig.Option{
		oauth2SessConfig.WithRefreshThreshold(config.IDPTokenRefreshThreshold),
	}
	if appConfig.SupportIDP {
		extraStoreOptions = append(extraStoreOptions,
			oauth2SessConfig.WithOIDCParams(s.oidcParam))
		if config.EnableNats {
			s.log.Info("Using NATS for session store")
			nc, nErr := nats.Connect(config.NatsURL)
			if nErr != nil {
				s.log.Fatal("Could not connect to NATS", log.ErrorField(nErr))
			}
			extraStoreOptions = append(extraStoreOptions,
				oauth2SessConfig.WithNATS(nc))
		}
	}
	s.sessionStore, err = sessionFactory.New[session.SessionStore, oauth2SessConfig.Option](
		oauth2SessConfig.SessionTypeOAuth2,
		[]session.Option{session.WithTimeout(config.SessionTimeout)},
		extraStoreOptions,
	)
	if err != nil {
		s.log.Fatal("Could not create session store", log.ErrorField(err))
	}
}

//nolint:lll // readability
func (s *grpcServer) SetupPendingAuthStateCache() {
	var err error
	extraStoreOptions := []oidcNatsImpl.Option{}

	commonOpts := []oidc.Option{oidc.WithTimeout(time.Second * 30)}
	//nolint:nestif // error handling
	if config.EnableNats {
		s.log.Info("Using NATS for pending logins")
		nc, nErr := nats.Connect(config.NatsURL)
		if nErr != nil {
			s.log.Fatal("Could not connect to NATS", log.ErrorField(nErr))
		}
		extraStoreOptions = append(extraStoreOptions,
			oidcNatsImpl.WithNATS(nc))
		s.pendingAuthStateCache, err = oidcFactory.New[oidc.PendingAuthStateCache, oidcNatsImpl.Option](
			oidcNatsImpl.PendingAuthStateCacheTypeNats,
			commonOpts,
			extraStoreOptions,
		)
		if err != nil {
			s.log.Fatal("Could not create pending auth state cache", log.ErrorField(err))
		}
	} else {
		s.pendingAuthStateCache, err = oidcFactory.New[oidc.PendingAuthStateCache, oidcSimplImpl.Option](
			oidcSimplImpl.PendingAuthStateCacheTypeSimple,
			commonOpts,
			nil,
		)
		if err != nil {
			s.log.Fatal("Could not create pending auth state cache", log.ErrorField(err))
		}
	}
}

func (s *grpcServer) SetupAuthInterceptor() {
	s.authInterceptor = authImpl.NewAuthInterceptor(
		auth.WithAdminToken(config.AdminToken),
		auth.WithTenantCache(s.tenantCache),
	)
}

func (s *grpcServer) SetupSessionInterceptor() {
	s.sessionInterceptor = sessionImpl.NewSessionInterceptor(s.sessionStore)
}

func (s *grpcServer) SetupConfigInterceptor() {
	s.configInterceptor = serverUtil.NewAppContextInterceptor(&appConfig)
}

func (s *grpcServer) SetupTraceIDInterceptor() {
	s.traceIDInterceptor = serverUtil.NewTraceIDInterceptor()
}

func (s *grpcServer) SetupRepositories() {
	s.repos = bobRepos.NewRepositoriesFromPool(s.pool)
}

func (s *grpcServer) SetupTransactionManager() {
	s.txManager = bobRepos.NewBobTransactionFromPool(s.pool)
}

//nolint:funlen // by design
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
	s.registerSettingsServer()
	s.registerAuthServer()
	s.registerUserServer()
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
				Handler: h2c.NewHandler(
					newCORS().Handler(s.mux),
					&http2.Server{
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
			Handler: h2c.NewHandler(
				newCORS().Handler(s.mux),
				&http2.Server{
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

func (s *grpcServer) registerAuthServer() {
	if !appConfig.SupportIDP {
		s.log.Debug("IDP based authentication disabled")
		return
	}
	if config.IDPClientID == "" ||
		config.IDPClientSecret == "" ||
		config.IDPIssuerURL == "" ||
		config.IDPCallbackURL == "" {

		s.log.Error("Missing IDP configuration. Cannot start auth service")
		return
	}

	authServerImpl := authServer.NewServer(
		authServer.WithTracer(s.tracer),
		authServer.WithSessionStore(s.sessionStore),
		authServer.WithOIDCParams(s.oidcParam),
		authServer.WithPendingAuthStateCache(s.pendingAuthStateCache),
	)
	path, handler := authv1connect.NewAuthServiceHandler(
		authServerImpl,
		connect.WithInterceptors(
			s.otel,
			s.traceIDInterceptor,
			s.configInterceptor,
			s.sessionInterceptor,
			s.authInterceptor,
		),
	)
	s.mux.Handle(path, handler)
	callbackURL, callbackHandler := authServerImpl.CallbackHandler()
	s.mux.Handle(callbackURL, callbackHandler)
}

func (s *grpcServer) registerUserServer() {
	// only used when IDP is enabled
	if !appConfig.SupportIDP {
		return
	}
	userService := user.NewServer(
		user.WithSessionStore(s.sessionStore),
		user.WithTracer(s.tracer), // Added
	)
	path, handler := userv1connect.NewUserServiceHandler(
		userService,
		connect.WithInterceptors(
			s.otel,
			s.traceIDInterceptor,
			s.configInterceptor,
			s.sessionInterceptor,
			s.authInterceptor),
	)

	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerSettingsServer() {
	settingsService := settings.NewServer(
		settings.WithTracer(s.tracer),
		settings.WithSupportsLogin(appConfig.SupportIDP),
	)
	path, handler := settingsv1connect.NewSettingsServiceHandler(
		settingsService,
		connect.WithInterceptors(
			s.otel,
			s.traceIDInterceptor,
			s.configInterceptor,
			s.sessionInterceptor,
			s.authInterceptor),
	)

	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerEventServer() {
	eventServerImpl := event.NewServer(
		event.WithRepositories(s.repos),
		event.WithTxManager(s.txManager),
		event.WithEventService(eventService.NewEventService(
			s.repos,
			s.txManager)),
		event.WithPermissionEvaluator(permission.NewPermissionEvaluator()),
		event.WithTracer(s.tracer), // Added
	)
	path, handler := eventv1connect.NewEventServiceHandler(
		eventServerImpl,
		connect.WithInterceptors(
			s.otel,
			s.traceIDInterceptor,
			s.configInterceptor,
			s.sessionInterceptor,
			s.authInterceptor,
		),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerAnalysisServer() {
	analysisService := analysis.NewServer(
		analysis.WithRepositories(s.repos),
		analysis.WithTransactionManager(s.txManager),
		analysis.WithPermissionEvaluator(permission.NewPermissionEvaluator()),
		analysis.WithTracer(s.tracer), // Added
	)
	path, handler := analysisv1connect.NewAnalysisServiceHandler(
		analysisService,
		connect.WithInterceptors(
			s.otel, s.traceIDInterceptor, s.configInterceptor, s.authInterceptor),
	)

	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerProviderServer() {
	providerService := provider.NewServer(
		provider.WithRepositories(s.repos),
		provider.WithTxManager(s.txManager),
		provider.WithEventLookup(s.eventLookup),
		provider.WithDataProxy(s.dataProxy),
		provider.WithTracer(s.tracer),
		provider.WithPermissionEvaluator(permission.NewPermissionEvaluator()))
	path, handler := providerv1connect.NewProviderServiceHandler(
		providerService,
		connect.WithInterceptors(
			s.otel,
			s.traceIDInterceptor,
			s.configInterceptor,
			s.sessionInterceptor,
			s.authInterceptor,
		),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerStateServer() {
	stateService := state.NewServer(
		state.WithRepositories(s.repos),
		state.WithTxManager(s.txManager),
		state.WithEventLookup(s.eventLookup), // to be removed?
		state.WithDataProxy(s.dataProxy),
		state.WithPermissionEvaluator(permission.NewPermissionEvaluator()),
		state.WithTracer(s.tracer), // Added
	)
	path, handler := racestatev1connect.NewRaceStateServiceHandler(
		stateService,
		connect.WithInterceptors(s.otel, s.traceIDInterceptor, s.authInterceptor),
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
		connect.WithInterceptors(s.otel, s.traceIDInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerTrackServer() {
	trackService := track.NewServer(
		track.WithTrackRepository(s.repos.Track()),
		track.WithPermissionEvaluator(permission.NewPermissionEvaluator()),
		track.WithTracer(s.tracer), // Added
	)
	path, handler := trackv1connect.NewTrackServiceHandler(
		trackService,
		connect.WithInterceptors(s.otel, s.traceIDInterceptor, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerTenantServer() {
	tenantService := tenant.NewServer(
		tenant.WithRepository(s.repos.Tenant()),
		tenant.WithTenantCache(s.tenantCache),
		tenant.WithPermissionEvaluator(permission.NewPermissionEvaluator()),
		tenant.WithTracer(s.tracer), // Added
	)
	path, handler := tenantv1connect.NewTenantServiceHandler(
		tenantService,
		connect.WithInterceptors(s.otel, s.traceIDInterceptor, s.authInterceptor),
	)
	s.mux.Handle(path, handler)
}

func (s *grpcServer) registerPredictServer() {
	predictService := predict.NewServer(
		predict.WithRepositories(s.repos),
		predict.WithEventLookup(s.eventLookup), // to be removed?
		predict.WithTracer(s.tracer),           // Added
	)
	path, handler := predictv1connect.NewPredictServiceHandler(
		predictService,
		connect.WithInterceptors(s.otel, s.traceIDInterceptor),
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
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
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
