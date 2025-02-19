package config

import "context"

// this holds the resolved configuration values from CLI
//
//nolint:lll // readablity
var (
	DB                   string // connection string for the database
	WaitForServices      string // duration to wait for other services to be ready
	LogLevel             string // sets the log level (zap log level values)
	LogFormat            string // text vs json
	LogConfig            string // path to log config file
	MigrationSourceUrl   string // location of migration files
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
	ProviderToken        string // token for data provider access
	AdminToken           string // token for admin access
	StaleDuration        string // duration after which an event is considered stale
	MaxConcurrentStreams int    // max number of concurrent streams per connection
	NatsUrl              string // nats server url
	EnableNats           bool   // enable nats
)

// Config holds the configuration values which are used by the application
type (
	contextKey struct{}
	Config     struct {
		PrintMessage   bool // if true, the message payload will be print on debug level
		SupportTenants bool // if true, tenant support is enabled
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
