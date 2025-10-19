# METADATA
# scope: package
# description: This policy defines the authorization rules for the iRacelog gRPC endpoints
# The input is a struct with the following fields:
#
# tenant: the name of the executing tenant
# action: the action to be performed (see  data.permissions)
# objectOwner: the tenant name owning the object
#
package iracelog.authz

default allow := false

# METADATA
# title: admin role allows everything
allow if has_admin_role

# METADATA
# title: check access which are independant of the object owner (no input.objectOwner)
#
allow if {
	not input.objectOwner
	input.action in user_permissions
}

# METADATA
# title: check access which are independant of the object owner (empty input.objectOwner)
allow if {
	input.objectOwner = ""
	input.action in user_permissions
}

# METADATA
# title: check access by scoped permission
allow if {
    has_scoped_permission
}

scoped_roles contains r if {
	some r, _ in input.scoped
}

# allowed_scopes contains r if {
# 	some role, scopes in input.scoped
# 	some s in scopes
# 	r := sprintf("%s_%s", [role, s])
# }
#
# has_scoped_role if {
# 	some r in scoped_roles
# 	sprintf("%s_%s", [r, input.objectOwner]) in allowed_scopes
# }

has_scoped_permission if {
	sprintf("%s_%s", [input.action, input.objectOwner]) in scoped_user_permissions
}


reachable_roles := graph.reachable(data.roles_graph, input.roles)
rr_scoped := graph.reachable(data.roles_graph, scoped_roles)

user_permissions contains permission if {
	some role in reachable_roles
	some permission in data.permissions[role]
}


scoped_user_permissions contains scoped_perm if {
	some role in rr_scoped
	some permission in data.permissions[role]
    some t in input.scoped[role]
    scoped_perm := sprintf("%s_%s", [permission, t])
}

has_admin_role if "admin" in reachable_roles
