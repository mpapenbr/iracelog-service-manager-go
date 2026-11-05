package util

import (
	"context"

	"connectrpc.com/connect/v2"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
)

// NewAppContextInterceptor returns an interceptor that adds the app config
// to the context
//
//nolint:whitespace // editor/linter issue
func NewAppContextInterceptor(cfg *config.Config) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(
			ctx context.Context,
			spec connect.Spec,
			stream connect.ServerStream,
		) error {
			return next(config.NewContext(ctx, cfg), spec, stream)
		}
	}
}
