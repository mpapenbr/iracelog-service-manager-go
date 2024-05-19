package permission

type StaticRoleManager struct {
	PermissionEvaluator
}

func NewStaticRoleManager() *StaticRoleManager {
	return &StaticRoleManager{}
}
