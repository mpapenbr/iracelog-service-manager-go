package log

import (
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"moul.io/zapfilter"
)

type Level = zapcore.Level

const (
	InfoLevel   Level = zap.InfoLevel   // 0, default level
	WarnLevel   Level = zap.WarnLevel   // 1
	ErrorLevel  Level = zap.ErrorLevel  // 2
	DPanicLevel Level = zap.DPanicLevel // 3, used in development log
	// PanicLevel logs a message, then panics
	PanicLevel Level = zap.PanicLevel // 4
	// FatalLevel logs a message, then calls os.Exit(1).
	FatalLevel Level = zap.FatalLevel // 5
	DebugLevel Level = zap.DebugLevel // -1
)

type Field = zap.Field

type Logger struct {
	l             *zap.Logger // zap ensure that zap.Logger is safe for concurrent use
	level         Level
	baseConfig    *zap.Config
	loggerConfigs map[string]string
}

func (l *Logger) Level() Level {
	return l.level
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.l.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.l.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.l.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.l.Error(msg, fields...)
}

func (l *Logger) DPanic(msg string, fields ...Field) {
	l.l.DPanic(msg, fields...)
}

func (l *Logger) Panic(msg string, fields ...Field) {
	l.l.Panic(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	l.l.Fatal(msg, fields...)
}

func (l *Logger) Log(lvl Level, msg string, fields ...Field) {
	l.l.Log(lvl, msg, fields...)
}

func (l *Logger) Named(name string) *Logger {
	level := l.level // default level in case of no match or no valid log level
	fullLoggerName := name
	if l.l.Name() != "" {
		fullLoggerName = l.l.Name() + "." + name
	}
	loggers := slices.Collect(maps.Keys(l.loggerConfigs))
	bestMatch := findBestMatch(loggers, fullLoggerName)
	if bestMatch != "" {
		if cfg, ok := l.loggerConfigs[bestMatch]; ok {
			if cfg != "" {
				lvl, _ := zap.ParseAtomicLevel(cfg)
				level = lvl.Level()
			}
		}

		myConfig := *l.baseConfig
		myConfig.Level = zap.NewAtomicLevelAt(level)
		lt, _ := myConfig.Build()
		return &Logger{
			l: zap.New(lt.Core(),
				zap.WithCaller(!l.baseConfig.DisableCaller),
				zap.AddStacktrace(zap.ErrorLevel),
				AddCallerSkip(1)).Named(fullLoggerName),

			level:         lt.Level(),
			baseConfig:    l.baseConfig,
			loggerConfigs: l.loggerConfigs,
		}
	}
	return &Logger{
		l:             l.l.Named(name),
		level:         level,
		baseConfig:    l.baseConfig,
		loggerConfigs: l.loggerConfigs,
	}
}

func (l *Logger) WithOptions(opts ...Option) *Logger {
	return &Logger{
		l:     l.l.WithOptions(opts...),
		level: l.level,
	}
}

// function variables for all field types
// in github.com/uber-go/zap/field.go

var (
	Skip        = zap.Skip
	Binary      = zap.Binary
	Bool        = zap.Bool
	Boolp       = zap.Boolp
	ByteString  = zap.ByteString
	Complex128  = zap.Complex128
	Complex128p = zap.Complex128p
	Complex64   = zap.Complex64
	Complex64p  = zap.Complex64p
	Float64     = zap.Float64
	Float64p    = zap.Float64p
	Float32     = zap.Float32
	Float32p    = zap.Float32p
	Int         = zap.Int
	Intp        = zap.Intp
	Int64       = zap.Int64
	Int64p      = zap.Int64p
	Int32       = zap.Int32
	Int32p      = zap.Int32p
	Int16       = zap.Int16
	Int16p      = zap.Int16p
	Int8        = zap.Int8
	Int8p       = zap.Int8p
	String      = zap.String
	Stringp     = zap.Stringp
	Uint        = zap.Uint
	Uintp       = zap.Uintp
	Uint64      = zap.Uint64
	Uint64p     = zap.Uint64p
	Uint32      = zap.Uint32
	Uint32p     = zap.Uint32p
	Uint16      = zap.Uint16
	Uint16p     = zap.Uint16p
	Uint8       = zap.Uint8
	Uint8p      = zap.Uint8p
	Uintptr     = zap.Uintptr
	Uintptrp    = zap.Uintptrp
	Reflect     = zap.Reflect
	Namespace   = zap.Namespace
	Stringer    = zap.Stringer
	Time        = zap.Time
	Timep       = zap.Timep
	Stack       = zap.Stack
	StackSkip   = zap.StackSkip
	Duration    = zap.Duration
	Durationp   = zap.Durationp
	Any         = zap.Any

	Info   = std.Info
	Warn   = std.Warn
	Error  = std.Error
	DPanic = std.DPanic
	Panic  = std.Panic
	Fatal  = std.Fatal
	Debug  = std.Debug
)

// not safe for concurrent use
func ResetDefault(l *Logger) {
	std = l
	Info = std.Info
	Warn = std.Warn
	Error = std.Error
	DPanic = std.DPanic
	Panic = std.Panic
	Fatal = std.Fatal
	Debug = std.Debug
}

var std = New(os.Stderr, InfoLevel, WithCaller(true), AddCallerSkip(1))

func Default() *Logger {
	return std
}

func ErrorField(err error) Field {
	return zap.Error(err)
}

type Option = zap.Option

var (
	AddCallerSkip = zap.AddCallerSkip
	WithCaller    = zap.WithCaller
	AddStacktrace = zap.AddStacktrace
)

type RotateOptions struct {
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
}

type LevelEnablerFunc func(lvl Level) bool

type TeeOption struct {
	Filename string
	Ropt     RotateOptions
	Lef      LevelEnablerFunc
}

// New create a new logger for production.
//
//nolint:dupl //yes, very similar to DevLogger
func New(writer io.Writer, level Level, opts ...Option) *Logger {
	if writer == nil {
		panic("the writer is nil")
	}
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02T15:04:05.000Z0700"))
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg.EncoderConfig),
		zapcore.AddSync(writer),
		level,
	)
	logger := &Logger{
		l:     zap.New(core, opts...),
		level: level,
	}
	return logger
}

func NewWithConfig(cfg *Config, level string) *Logger {
	if level != "" {
		lvl, _ := zap.ParseAtomicLevel(level)
		cfg.Zap.Level = lvl
	}

	lt, _ := cfg.Zap.Build()
	myCore := lt.Core()
	if cfg.Filters != nil {
		// concatenate items to one string
		var filters string
		for _, filter := range cfg.Filters {
			filters += filter + " "
		}
		lt = zap.New(zapfilter.NewFilteringCore(
			myCore,
			zapfilter.MustParseRules(filters)),
		)
	}

	logger := &Logger{
		l: zap.New(lt.Core(),
			zap.WithCaller(!cfg.Zap.DisableCaller),
			zap.AddStacktrace(zap.ErrorLevel),
			AddCallerSkip(1)),
		level:         lt.Level(),
		baseConfig:    &cfg.Zap,
		loggerConfigs: cfg.Loggers,
	}
	return logger
}

// DevLogger create a new logger for development.
//
//nolint:dupl //yes, very similar to New
func DevLogger(writer io.Writer, level Level, opts ...Option) *Logger {
	if writer == nil {
		panic("the writer is nil")
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02T15:04:05.000"))
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg.EncoderConfig),
		zapcore.AddSync(writer),
		level,
	)

	logger := &Logger{
		l:     zap.New(core, opts...),
		level: level,
	}
	return logger
}

func ParseLevel(levelStr string) (Level, error) {
	return zapcore.ParseLevel(levelStr)
}

func (l *Logger) Sync() error {
	return l.l.Sync()
}

func Sync() error {
	if std != nil {
		return std.Sync()
	}
	return nil
}

func findBestMatch(stringsList []string, query string) string {
	var bestMatch string
	queryParts := strings.Split(query, ".")

	for _, s := range stringsList {
		sParts := strings.Split(s, ".")

		// we can only match if the query has at least as many parts as the string
		if len(sParts) <= len(queryParts) {
			matches := true
			for i := range sParts {
				pattern := "^" + sParts[i] + "$"
				matched, _ := regexp.MatchString(pattern, queryParts[i])
				if !matched {
					matches = false
					break
				}
			}

			if matches &&
				(bestMatch == "" || len(sParts) > len(strings.Split(bestMatch, "."))) {

				bestMatch = s
			}
		}
	}

	return bestMatch
}
