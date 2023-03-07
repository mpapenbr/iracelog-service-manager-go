package config

// this holds the resolved configuration values
var (
	DB          string // connection string for the database
	URL         string // URL of WAMP server
	Realm       string // realm to use
	Password    string // password for backend
	LogLevel    string // sets the log level (zap log level values)
	SQLLogLevel string // sets the log level for sql subsystem
	LogFormat   string // text vs json

)
