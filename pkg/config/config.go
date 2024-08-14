package config

// this holds the resolved configuration values from CLI
//
//nolint:lll // readablity
var (
	DB                   string // connection string for the database
	URL                  string // URL of WAMP server
	Realm                string // realm to use
	Password             string // password for backend
	WaitForServices      string // duration to wait for other services to be ready
	LogLevel             string // sets the log level (zap log level values)
	SQLLogLevel          string // sets the log level for sql subsystem
	LogFormat            string // text vs json
	LogConfig            string // path to log config file
	MigrationSourceUrl   string // location of migration files
	EnableTelemetry      bool   // enable telemetry
	TelemetryEndpoint    string // endpoint for telemetry
	ProfilingPort        int    // port for profiling
	PrintMessage         bool   // if true, the message payload will be print on debug level
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

)

// Config holds the configuration values which are used by the application
type Config struct {
	PrintMessage bool // if true, the message payload will be print on debug level
}
