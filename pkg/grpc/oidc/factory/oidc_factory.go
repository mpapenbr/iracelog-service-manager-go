package factory

import (
	"errors"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc"
)

type CacheType string

var (
	ErrTypeNotSupported = errors.New("oidc pending auth state cache type not supported")
	ErrWrongCreator     = errors.New("oidc pending auth state cache wrong creator")
)

//nolint:lll //readability
type Creator[S oidc.PendingAuthStateCache, ImplOpt any] func([]oidc.Option, []ImplOpt) (S, error)

var registry = map[CacheType]any{}

// Register a new implementation generically
//
//nolint:whitespace //editor/linter issue
func Register[S oidc.PendingAuthStateCache, ImplOpt any](
	key CacheType, creator Creator[S, ImplOpt],
) {
	registry[key] = creator
}

// Create a new instance
//
//nolint:whitespace //editor/linter issue
func New[S oidc.PendingAuthStateCache, ImplOpt any](
	key CacheType,
	common []oidc.Option,
	specific []ImplOpt,
) (S, error) {
	entry, ok := registry[key]
	if !ok {
		var zero S
		return zero, ErrTypeNotSupported
	}
	creator, ok := entry.(Creator[S, ImplOpt])
	if !ok {
		var zero S
		return zero, ErrWrongCreator
	}
	return creator(common, specific)
}
