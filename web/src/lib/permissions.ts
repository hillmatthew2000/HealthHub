import type { Role } from '@/api/types'

// Mirrors the access policy in internal/auth/roles.go. The API enforces it;
// the UI only uses it to hide navigation and actions a role cannot use.

export type Resource = 'patients' | 'observations' | 'users' | 'audit-log'
export type Action = 'create' | 'read' | 'update' | 'delete'

const crud: Action[] = ['create', 'read', 'update', 'delete']

const permissions: Record<Role, Partial<Record<Resource, Action[]>>> = {
  admin: {
    patients: crud,
    observations: crud,
    users: crud,
    'audit-log': ['read'],
  },
  practitioner: {
    patients: ['create', 'read', 'update'],
    observations: ['create', 'read', 'update'],
  },
  nurse: {
    patients: ['read'],
    observations: ['read'],
  },
  'lab-tech': {
    patients: ['read'],
    observations: ['create', 'read', 'update'],
  },
}

/** Reports whether any of roles grants action on resource. Unknown roles grant nothing. */
export function can(roles: readonly string[] | undefined, resource: Resource, action: Action): boolean {
  return (roles ?? []).some((r) => permissions[r as Role]?.[resource]?.includes(action) ?? false)
}

export type Permission = readonly [Resource, Action]

export const ROLE_LABELS: Record<Role, string> = {
  admin: 'Administrator',
  practitioner: 'Practitioner',
  nurse: 'Nurse',
  'lab-tech': 'Lab technician',
}

export const ROLE_DESCRIPTIONS: Record<Role, string> = {
  admin: 'Full access, including accounts and the audit trail.',
  practitioner: 'Creates and updates patients and observations.',
  nurse: 'Reads patients and observations.',
  'lab-tech': 'Reads patients; creates and updates observations.',
}
