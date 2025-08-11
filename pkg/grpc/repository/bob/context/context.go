package context

import (
	"context"

	"github.com/stephenafamo/bob"
)

type bopContextKey struct{}

func NewContext(ctx context.Context, executor bob.Executor) context.Context {
	return context.WithValue(ctx, bopContextKey{}, executor)
}

func FromContext(ctx context.Context) bob.Executor {
	if ctx == nil {
		return nil
	}
	if executor, ok := ctx.Value(bopContextKey{}).(bob.Executor); ok {
		return executor
	}
	return nil
}
