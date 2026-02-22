package log

import (
	"context"
	"maps"
	"regexp"
	"slices"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/contrib/processors/minsev"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"moul.io/zapfilter"
)

type (
	Level = zapcore.Level
	Field = zap.Field
	// note: if ctx is set it will be added as a log.Field for all log calls
	// the use case is to add the span context to the log,
	//  so that the traceID is included in the log output
	// for example:
	// log.Ctx(spanCtx).Debug("message") will add the span context to the log entry
	Logger struct {
		l             *zap.Logger // zap ensure that zap.Logger is safe for concurrent use
		level         Level
		zapConfig     *zap.Config
		loggerConfigs map[string]string
		myCfg         *loggerConfig
		ctx           context.Context
	}
	LevelEnablerFunc func(lvl Level) bool

	TeeOption struct {
		Filename string
		Ropt     RotateOptions
		Lef      LevelEnablerFunc
	}
)

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

var std = New()

func New(opts ...ConfigOption) *Logger {
	myCfg := newLoggerConfig(opts...)
	cfg := myCfg.cfg

	if myCfg.level != "" {
		lvl, _ := zap.ParseAtomicLevel(myCfg.level)
		cfg.Zap.Level = lvl
	}

	zapLogger, _ := cfg.Zap.Build()

	zapLogger = combinedCores(zapLogger, "", myCfg, cfg.Zap.Level.Level())

	if cfg.Filters != nil {
		// concatenate items to one string
		var filters string
		for _, filter := range cfg.Filters {
			filters += filter + " "
		}
		zapLogger = zap.New(zapfilter.NewFilteringCore(
			zapLogger.Core(),
			zapfilter.MustParseRules(filters)),
		)
	}
	logger := &Logger{
		l:             zapLogger,
		level:         cfg.Zap.Level.Level(),
		zapConfig:     &cfg.Zap,
		loggerConfigs: cfg.Loggers,
		myCfg:         myCfg,
	}
	return logger
}

func Default() *Logger {
	return std
}

func ErrorField(err error) Field {
	return zap.Error(err)
}

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

func (l *Logger) Level() Level {
	return l.level
}

func (l *Logger) WithCtx(ctx context.Context) *Logger {
	ret := &Logger{
		l:             l.l,
		level:         l.level,
		zapConfig:     l.zapConfig,
		loggerConfigs: l.loggerConfigs,
		myCfg:         l.myCfg,
		ctx:           ctx,
	}
	return ret
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.l.Debug(msg, l.addContext(fields)...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.l.Info(msg, l.addContext(fields)...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.l.Warn(msg, l.addContext(fields)...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.l.Error(msg, l.addContext(fields)...)
}

func (l *Logger) DPanic(msg string, fields ...Field) {
	l.l.DPanic(msg, l.addContext(fields)...)
}

func (l *Logger) Panic(msg string, fields ...Field) {
	l.l.Panic(msg, l.addContext(fields)...)
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	l.l.Fatal(msg, l.addContext(fields)...)
}

func (l *Logger) Log(lvl Level, msg string, fields ...Field) {
	l.l.Log(lvl, msg, l.addContext(fields)...)
}

func (l *Logger) addContext(fields []Field) []Field {
	if l.ctx == nil {
		return fields
	}
	return append(fields, zap.Any("context", l.ctx))
}

func (l *Logger) Named(name string) *Logger {
	level := l.level // default level in case of no match or no valid log level
	fullLoggerName := name
	if l.l.Name() != "" {
		fullLoggerName = l.l.Name() + "." + name
	}
	zapL := l.l.Named(name)
	loggers := slices.Collect(maps.Keys(l.loggerConfigs))
	bestMatch := findBestMatch(loggers, fullLoggerName)
	if bestMatch != "" {
		if cfg, ok := l.loggerConfigs[bestMatch]; ok {
			if cfg != "" {
				lvl, _ := zap.ParseAtomicLevel(cfg)
				level = lvl.Level()
			}
		}

		myConfig := *l.zapConfig
		myConfig.Level = zap.NewAtomicLevelAt(level)

		lt, _ := myConfig.Build()
		zapL = combinedCores(lt.Named(fullLoggerName), fullLoggerName, l.myCfg, level)
	}
	return &Logger{
		l:             zapL,
		level:         level,
		zapConfig:     l.zapConfig,
		loggerConfigs: l.loggerConfigs,
		myCfg:         l.myCfg,
	}
}

// this core is used to remove fields containing a context.Context value
// we need this to prevent the span context from being logged
// we need the span context for the otelzap logger to output the traceID
type contextIgnoringCore struct {
	zapcore.Core
}

func (c *contextIgnoringCore) With(fields []zapcore.Field) zapcore.Core {
	return &contextIgnoringCore{Core: c.Core.With(fields)}
}

//nolint:gocritic // external interface
func (c *contextIgnoringCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	cleanedFields := make([]zapcore.Field, 0, len(fields))
	for i := range fields {
		f := fields[i]
		if _, ok := f.Interface.(context.Context); ok {
			continue
		}
		cleanedFields = append(cleanedFields, f)
	}
	return c.Core.Write(ent, cleanedFields)
}

//nolint:whitespace,gocritic // editor/linter issue
func (c *contextIgnoringCore) Check(
	ent zapcore.Entry,
	ce *zapcore.CheckedEntry,
) *zapcore.CheckedEntry {
	// we just need to set add ourselves to the core
	ret := ce.AddCore(ent, c)
	return ret
}

//nolint:whitespace // editor/linter issue
func combinedCores(
	zl *zap.Logger,
	name string,
	myCfg *loggerConfig,
	level Level,
) *zap.Logger {
	useCores := make([]zapcore.Core, 0)
	if myCfg.telemetry != nil {
		otelSeverity := &minsevSeverity{convertLevel(level)}
		customLogger := myCfg.telemetry.CustomizedLogger(func(
			exporter sdklog.Exporter,
			downstream sdklog.Processor,
		) sdklog.LoggerProviderOption {
			proc := minsev.NewLogProcessor(downstream, otelSeverity)
			return sdklog.WithProcessor(proc)
		})

		useCores = append(useCores, otelzap.NewCore(
			name, otelzap.WithLoggerProvider(customLogger)))
	}
	if myCfg.useZap {
		if myCfg.removeContextFields {
			useCores = append(useCores, &contextIgnoringCore{
				Core: zl.Core(),
			})
		} else {
			useCores = append(useCores, zl.Core())
		}
	}
	combinedCore := zapcore.NewTee(
		useCores...,
	)

	ret := zap.New(combinedCore,
		zap.WithCaller(!myCfg.cfg.Zap.DisableCaller),
		zap.AddStacktrace(zap.ErrorLevel),
		AddCallerSkip(1))

	return ret
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
