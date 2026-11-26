// Types mirror the Go models in internal/models and internal/handlers.

export type Role = 'admin' | 'practitioner' | 'nurse' | 'lab-tech'
export const ROLES: readonly Role[] = ['admin', 'practitioner', 'nurse', 'lab-tech']

/** One rejected input field in a problem response. */
export interface FieldError {
  field: string
  message: string
}

/** RFC 9457 problem details with the API's code, requestId and errors extensions. */
export interface Problem {
  type: string
  title: string
  status: number
  detail?: string
  instance?: string
  code: string
  requestId?: string
  errors?: FieldError[]
}

export interface Page<T> {
  data: T[]
  page: number
  limit: number
  total: number
  totalPages: number
}

export interface PageQuery {
  page?: number
  limit?: number
}

export interface HealthStatus {
  status: string
  version?: string
  checks?: Record<string, string>
}

// Accounts

export interface UserInfo {
  id: string
  email: string
  firstName: string
  lastName: string
  roles: Role[]
  active: boolean
  lastLoginAt?: string
  createdAt: string
}

export interface AuthResponse {
  token: string
  expiresAt: string
  user: UserInfo
}

export interface LoginRequest {
  email: string
  password: string
}

export interface ChangePasswordRequest {
  currentPassword: string
  newPassword: string
}

export interface CreateUserRequest {
  email: string
  password: string
  firstName: string
  lastName: string
  roles: Role[]
}

export interface UpdateUserRequest {
  roles?: Role[]
  active?: boolean
}

// FHIR-inspired resources

export type Gender = 'male' | 'female' | 'other' | 'unknown'
export const GENDERS: readonly Gender[] = ['male', 'female', 'other', 'unknown']
export const NAME_USES = ['usual', 'official', 'temp', 'nickname', 'anonymous', 'old', 'maiden'] as const
export const TELECOM_SYSTEMS = ['phone', 'fax', 'email', 'pager', 'url', 'sms', 'other'] as const
export const TELECOM_USES = ['home', 'work', 'temp', 'old', 'mobile'] as const
export const ADDRESS_USES = ['home', 'work', 'temp', 'old', 'billing'] as const
export const ADDRESS_TYPES = ['postal', 'physical', 'both'] as const

export interface Period {
  start?: string
  end?: string
}

export interface HumanName {
  use?: string
  family: string
  given: string[]
  prefix?: string[]
  suffix?: string[]
}

export interface ContactPoint {
  system: string
  value: string
  use?: string
  rank?: number
}

export interface Address {
  use?: string
  type?: string
  text?: string
  line?: string[]
  city?: string
  district?: string
  state?: string
  postalCode?: string
  country?: string
  period?: Period
}

export interface Patient {
  id: string
  active: boolean
  name: HumanName[]
  gender: Gender
  birthDate?: string
  telecom?: ContactPoint[]
  address?: Address[]
  createdAt: string
  updatedAt: string
  createdBy?: string
}

/** The writable part of a patient; the server ignores the other fields. */
export type PatientInput = Pick<Patient, 'active' | 'name' | 'gender' | 'birthDate' | 'telecom' | 'address'>

export interface PatientQuery extends PageQuery {
  family?: string
  telecom?: string
  birthdate?: string
  gender?: Gender
  active?: boolean
}

export const OBSERVATION_STATUSES = [
  'registered',
  'preliminary',
  'final',
  'amended',
  'corrected',
  'cancelled',
  'entered-in-error',
  'unknown',
] as const
export type ObservationStatus = (typeof OBSERVATION_STATUSES)[number]

export interface Coding {
  system?: string
  version?: string
  code?: string
  display?: string
  userSelected?: boolean
}

export interface CodeableConcept {
  coding?: Coding[]
  text?: string
}

export interface Reference {
  reference?: string
  type?: string
  display?: string
}

export interface Quantity {
  value?: number
  comparator?: string
  unit?: string
  system?: string
  code?: string
}

export interface Annotation {
  authorString?: string
  time?: string
  text: string
}

export interface ObservationCategory {
  coding: Coding[]
  text?: string
}

interface ObservationCore {
  id: string
  status: ObservationStatus
  category?: ObservationCategory[]
  code: CodeableConcept
  subject: Reference
  effectiveDateTime: string
  issued?: string
  valueQuantity?: Quantity
  valueCodeableConcept?: CodeableConcept
  valueString?: string
  valueBoolean?: boolean
  valueInteger?: number
  interpretation?: CodeableConcept[]
  note?: Annotation[]
  createdAt: string
  updatedAt: string
  createdBy?: string
}

/**
 * The API accepts more FHIR elements (reference ranges, components and so
 * on). The UI does not edit them but keeps them intact on update.
 */
type ExtraElements = { [extra: string]: unknown }

export type Observation = ObservationCore & ExtraElements

export type ObservationInput = Omit<ObservationCore, 'id' | 'createdAt' | 'updatedAt' | 'createdBy'> & ExtraElements

export interface ObservationQuery extends PageQuery {
  status?: ObservationStatus
  category?: string
  code?: string
  from?: string
  to?: string
  patient?: string
}

// Audit trail

export const AUDIT_RESOURCE_TYPES = ['patient', 'observation', 'user'] as const
export type AuditResourceType = (typeof AUDIT_RESOURCE_TYPES)[number]

export interface AuditEvent {
  id: number
  occurredAt: string
  actorId?: string
  action: string
  resourceType: string
  resourceIds: string[]
  requestId?: string
  clientIp?: string
}

export interface AuditQuery extends PageQuery {
  actor?: string
  resource?: string
  resourceType?: AuditResourceType
  from?: string
  to?: string
}
