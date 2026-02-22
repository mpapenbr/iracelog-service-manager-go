package log

import (
	otellog "go.opentelemetry.io/otel/log"
	"go.uber.org/zap/zapcore"
)

// use this to control minsev LogProcesso
type minsevSeverity struct{ severity otellog.Severity }

func (m *minsevSeverity) Severity() otellog.Severity { return m.severity }

func convertLevel(level zapcore.Level) otellog.Severity {
	switch level {
	case zapcore.DebugLevel:
		return otellog.SeverityDebug
	case zapcore.InfoLevel:
		return otellog.SeverityInfo
	case zapcore.WarnLevel:
		return otellog.SeverityWarn
	case zapcore.ErrorLevel:
		return otellog.SeverityError
	case zapcore.DPanicLevel:
		return otellog.SeverityFatal1
	case zapcore.PanicLevel:
		return otellog.SeverityFatal2
	case zapcore.FatalLevel:
		return otellog.SeverityFatal3
	case zapcore.InvalidLevel:
		return otellog.SeverityUndefined
	default:
		return otellog.SeverityUndefined
	}
}
