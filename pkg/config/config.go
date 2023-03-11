package config

// this holds the resolved configuration values
var (
	DB              string // connection string for the database
	URL             string // URL of WAMP server
	Realm           string // realm to use
	Password        string // password for backend
	WaitForServices string // duration to wait for other services to be ready
	LogLevel        string // sets the log level (zap log level values)
	SQLLogLevel     string // sets the log level for sql subsystem
	LogFormat       string // text vs json

)
