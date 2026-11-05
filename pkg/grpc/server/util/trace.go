package util

import (
	"context"

	"connectrpc.com/connect/v2"
	"go.opentelemetry.io/otel/trace"
)

const (
	traceIDHeader = "X-Trace-ID"
)

// NewTraceIDInterceptor returns an interceptor that adds the trace id (if present)
// to the response header
//
//nolint:whitespace // editor/linter issue
func NewTraceIDInterceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(
			ctx context.Context,
			spec connect.Spec,
			stream connect.ServerStream,
		) error {
			span := trace.SpanFromContext(ctx)
			if span != nil && span.SpanContext().IsValid() {
				if info, ok := connect.CallInfoForServerContext(ctx); ok {
					info.ResponseHeader().Set(traceIDHeader,
						span.SpanContext().TraceID().String())
				}
			}
			return next(ctx, spec, stream)
		}
	}
}
