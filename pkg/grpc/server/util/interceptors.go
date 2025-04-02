package util

import (
	"context"

	"connectrpc.com/connect"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
)

type configInjector struct {
	config *config.Config
}

func NewAppContextInterceptor(config *config.Config) connect.Interceptor {
	return &configInjector{config: config}
}

//nolint:whitespace // better readability
func (i *configInjector) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		return next(config.NewContext(ctx, i.config), req)
	})
}

//
//nolint:lll,whitespace // readablity, editor/linter
func (i *configInjector) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

//
//nolint:lll,whitespace // readablity, editor/linter
func (i *configInjector) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		return next(config.NewContext(ctx, i.config), conn)
	})
}
