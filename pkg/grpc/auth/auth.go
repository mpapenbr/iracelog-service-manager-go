package auth

import "errors"

type Role int

const (
	RoleAdmin Role = iota
	RoleProvider
)

var ErrPermissionDenied = errors.New("permission denied")

type Principal interface {
	Name() string
}

type Authentication interface {
	Principal() Principal
	Roles() []Role
}
