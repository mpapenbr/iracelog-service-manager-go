package config

import (
	"context"
	"time"
)

// this holds the resolved configuration values from CLI
//
//nolint:lll // readablity
var (
	DB                   string // connection string for the database
	WaitForServices      string // duration to wait for other services to be ready
	LogLevel             string // sets the log level (zap log level values)
	LogFormat            string // text vs json
	LogConfig            string // path to log config file
	MigrationSourceURL   string // location of migration files
	EnableTelemetry      bool   // enable telemetry
	TelemetryEndpoint    string // endpoint for telemetry
	ProfilingPort        int    // port for profiling
	GrpcServerAddr       string // listen addr for gRPC server (insecure)
	TLSServerAddr        string // listen addr for gRPC server (tls)
	TLSCertFile          string // path to TLS certificate
	TLSKeyFile           string // path to TLS key
	TLSCAFile            string // path to TLS CA
	TraefikCerts         string // path to traefik certs file
	TraefikCertDomain    string // the domain to lookup within the traefik certs
	AdminToken           string // token for admin access
	StaleDuration        string // duration after which an event is considered stale
	MaxConcurrentStreams int    // max number of concurrent streams per connection
	NatsURL              string // nats server url
	EnableNats           bool   // enable nats

	// IDPConfig 		IDPConfig
	IDPClientID              string        // oauth2 client id
	IDPClientSecret          string        // oauth2 client secret
	IDPIssuerURL             string        // used to initialize oidc provider
	IDPCallbackURL           string        // callback URL for oauth2 in AuthCode flow
	IDPTokenRefreshThreshold time.Duration // validate active sessions against IDP
	SessionTimeout           time.Duration // remove session after inactivity

)

// Config holds the configuration values which are used by the application
type (
	contextKey struct{}
	Config     struct {
		PrintMessage   bool // if true, the message payload will be print on debug level
		SupportTenants bool // if true, tenant support is enabled
		SupportIDP     bool // if true, identity provider based authentication is enabled
	}

	IDPConfig struct {
		IssuerURL    string
		ClientID     string
		ClientSecret string
		CallbackURL  string
	}
)

var key = contextKey{}

func NewContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, key, cfg)
}

func FromContext(ctx context.Context) *Config {
	if ctx == nil {
		return nil
	}
	if cfg, ok := ctx.Value(key).(*Config); ok {
		return cfg
	}
	return nil
}
