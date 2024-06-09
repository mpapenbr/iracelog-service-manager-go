package config

// this holds the resolved configuration values from CLI
var (
	DB                 string // connection string for the database
	URL                string // URL of WAMP server
	Realm              string // realm to use
	Password           string // password for backend
	WaitForServices    string // duration to wait for other services to be ready
	LogLevel           string // sets the log level (zap log level values)
	SQLLogLevel        string // sets the log level for sql subsystem
	LogFormat          string // text vs json
	MigrationSourceUrl string // location of migration files
	EnableTelemetry    bool   // enable telemetry
	TelemetryEndpoint  string // endpoint for telemetry
	ProfilingPort      int    // port for profiling
	PrintMessage       bool   // if true, the message payload will be print on debug level
	GrpcServerAddr     string // listen addr for gRPC server
	ProviderToken      string // token for data provider access
	AdminToken         string // token for admin access
	StaleDuration      string // duration after which an event is considered stale

)

// Config holds the configuration values which are used by the application
type Config struct {
	PrintMessage bool // if true, the message payload will be print on debug level
}
