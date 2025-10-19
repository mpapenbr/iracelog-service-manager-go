package factory

import (
	"errors"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
)

type SessionType string

var (
	ErrSessionStoreTypeNotSupported = errors.New("session store type not supported")
	ErrSessionStoreWrongCreator     = errors.New("session store wrong creator")
)

//nolint:lll //readability
type Creator[S session.SessionStore, ImplOpt any] func([]session.Option, []ImplOpt) (S, error)

var registry = map[SessionType]any{}

// Register a new implementation generically
//
//nolint:whitespace //editor/linter issue
func Register[S session.SessionStore, ImplOpt any](
	key SessionType, creator Creator[S, ImplOpt],
) {
	registry[key] = creator
}

// Create a new instance
//
//nolint:whitespace //editor/linter issue
func New[S session.SessionStore, ImplOpt any](
	key SessionType,
	common []session.Option,
	specific []ImplOpt,
) (S, error) {
	entry, ok := registry[key]
	if !ok {
		var zero S
		return zero, ErrSessionStoreTypeNotSupported
	}
	creator, ok := entry.(Creator[S, ImplOpt])
	if !ok {
		var zero S
		return zero, ErrSessionStoreWrongCreator
	}
	return creator(common, specific)
}
