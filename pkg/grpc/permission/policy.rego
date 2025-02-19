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
# title: check access by permission and validate tenant is object owner
allow if {
	input.objectOwner == input.tenant
	input.action in user_permissions
}

reachable_roles := graph.reachable(data.roles_graph, input.roles)

user_permissions contains permission if {
	some role in reachable_roles
	some permission in data.permissions[role]
}

has_admin_role if "admin" in reachable_roles
