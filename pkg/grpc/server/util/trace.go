package util

import (
	"context"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/trace"
)

type traceIDInjector struct{}

func NewTraceIDInterceptor() connect.Interceptor {
	return &traceIDInjector{}
}

const (
	traceIDHeader = "X-Trace-ID"
)

//nolint:whitespace // better readability
func (i *traceIDInjector) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.SpanContext().IsValid() {
			traceID := span.SpanContext().TraceID().String()
			res, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			res.Header().Set(traceIDHeader, traceID)
			return res, nil
		}
		return next(ctx, req)
	})
}

//
//nolint:lll,whitespace // readablity, editor/linter
func (i *traceIDInjector) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

//
//nolint:lll,whitespace // readablity, editor/linter
func (i *traceIDInjector) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.SpanContext().IsValid() {
			traceID := span.SpanContext().TraceID().String()
			conn.ResponseHeader().Set(traceIDHeader, traceID)
		}
		return next(ctx, conn)
	})
}
