package permission

import (
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

type Permission string

const (
	PermissionRegisterEvent   Permission = "register-event"
	PermissionDeleteEvent     Permission = "delete-event"
	PermissionUpdateEvent     Permission = "update-event"
	PermissionUnregisterEvent Permission = "unregister-event"
	PermissionPostRacedata    Permission = "post-racedata"
)

const (
	PermissionCreateTenant Permission = "create-tenant"
	PermissionDeleteTenant Permission = "delete-tenant"
	PermissionUpdateTenant Permission = "update-tenant"
	PermissionReadTenant   Permission = "read-tenant"
)

const (
	PermissionCreateTrack Permission = "create-track"
	PermissionDeleteTrack Permission = "delete-track"
	PermissionUpdateTrack Permission = "update-track"
)

// collection of admin specific permissions
const (
	PermissionAdminUnregisterAllEvents Permission = "unregister-all-events"
)

type PermissionEvaluator interface {
	HasPermission(auth auth.Authentication, perm Permission) bool
	HasObjectPermission(auth auth.Authentication, perm Permission, objectOwner string) bool
}

func NewPermissionEvaluator() PermissionEvaluator {
	if ret, err := NewOpaPermissionEvaluator(); err != nil {
		log.Default().Error("failed to create permission evaluator", log.ErrorField(err))
		return nil
	} else {
		return ret
	}
}
