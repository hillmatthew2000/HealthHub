package auth

import "slices"

// Role is a coarse grained user role. Permissions are attached to roles in
// code (see permissions below) so the access policy is reviewable in one
// place and cannot drift between environments.
type Role string

// Supported roles.
const (
	RoleAdmin        Role = "admin"
	RolePractitioner Role = "practitioner"
	RoleNurse        Role = "nurse"
	RoleLabTech      Role = "lab-tech"
)

// AllRoles lists every supported role.
func AllRoles() []Role {
	return []Role{RoleAdmin, RolePractitioner, RoleNurse, RoleLabTech}
}

// Valid reports whether r is a supported role.
func (r Role) Valid() bool {
	return slices.Contains(AllRoles(), r)
}

// Resource is a protected resource type.
type Resource string

// Protected resources.
const (
	ResourcePatients     Resource = "patients"
	ResourceObservations Resource = "observations"
	ResourceUsers        Resource = "users"
	ResourceAuditLog     Resource = "audit-log"
)

// Action is an operation on a resource.
type Action string

// Actions.
const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

var crud = []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete}

// permissions is the access policy. Anything not listed is denied.
var permissions = map[Role]map[Resource][]Action{
	RoleAdmin: {
		ResourcePatients:     crud,
		ResourceObservations: crud,
		ResourceUsers:        crud,
		ResourceAuditLog:     {ActionRead},
	},
	RolePractitioner: {
		ResourcePatients:     {ActionCreate, ActionRead, ActionUpdate},
		ResourceObservations: {ActionCreate, ActionRead, ActionUpdate},
	},
	RoleNurse: {
		ResourcePatients:     {ActionRead},
		ResourceObservations: {ActionRead},
	},
	RoleLabTech: {
		ResourcePatients:     {ActionRead},
		ResourceObservations: {ActionCreate, ActionRead, ActionUpdate},
	},
}

// Can reports whether any of roles grants action on resource.
func Can(roles []Role, resource Resource, action Action) bool {
	for _, r := range roles {
		if slices.Contains(permissions[r][resource], action) {
			return true
		}
	}
	return false
}
