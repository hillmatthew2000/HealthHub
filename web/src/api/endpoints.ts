import type { ApiClient, Query } from './client'
import type {
  AuditEvent,
  AuditQuery,
  AuthResponse,
  ChangePasswordRequest,
  CreateUserRequest,
  HealthStatus,
  LoginRequest,
  Observation,
  ObservationInput,
  ObservationQuery,
  Page,
  PageQuery,
  Patient,
  PatientInput,
  PatientQuery,
  UpdateUserRequest,
  UserInfo,
} from './types'

const V1 = '/api/v1'

function seg(id: string): string {
  return encodeURIComponent(id)
}

export interface HealthResult extends HealthStatus {
  ok: boolean
}

/** Typed wrappers for every HealthHub API route the UI uses. */
export function createEndpoints(client: ApiClient) {
  return {
    health: {
      /** GET /health/ready. Health lives at the root, not under /api/v1; a 503 still carries a status body. */
      async ready(signal?: AbortSignal): Promise<HealthResult> {
        const { response, body } = await client.send('GET', '/health/ready', { auth: false, signal })
        const status = (body ?? {}) as Partial<HealthStatus>
        return {
          ok: response.ok,
          status: typeof status.status === 'string' ? status.status : response.ok ? 'ready' : 'unavailable',
          version: status.version,
          checks: status.checks,
        }
      },
    },
    auth: {
      login: (req: LoginRequest) => client.post<AuthResponse>(`${V1}/auth/login`, req, { auth: false }),
      refresh: () => client.post<AuthResponse>(`${V1}/auth/refresh`),
      profile: () => client.get<UserInfo>(`${V1}/auth/profile`),
      changePassword: (req: ChangePasswordRequest) => client.post<void>(`${V1}/auth/change-password`, req),
    },
    patients: {
      list: (q: PatientQuery = {}) => client.get<Page<Patient>>(`${V1}/patients`, { query: q as Query }),
      get: (id: string) => client.get<Patient>(`${V1}/patients/${seg(id)}`),
      create: (p: PatientInput) => client.post<Patient>(`${V1}/patients`, p),
      update: (id: string, p: PatientInput) => client.put<Patient>(`${V1}/patients/${seg(id)}`, p),
      remove: (id: string) => client.delete(`${V1}/patients/${seg(id)}`),
      observations: (id: string, q: Omit<ObservationQuery, 'patient'> = {}) =>
        client.get<Page<Observation>>(`${V1}/patients/${seg(id)}/observations`, { query: q as Query }),
    },
    observations: {
      list: (q: ObservationQuery = {}) => client.get<Page<Observation>>(`${V1}/observations`, { query: q as Query }),
      get: (id: string) => client.get<Observation>(`${V1}/observations/${seg(id)}`),
      create: (o: ObservationInput) => client.post<Observation>(`${V1}/observations`, o),
      update: (id: string, o: ObservationInput) => client.put<Observation>(`${V1}/observations/${seg(id)}`, o),
      remove: (id: string) => client.delete(`${V1}/observations/${seg(id)}`),
    },
    users: {
      list: (q: PageQuery = {}) => client.get<Page<UserInfo>>(`${V1}/users`, { query: q as Query }),
      get: (id: string) => client.get<UserInfo>(`${V1}/users/${seg(id)}`),
      create: (u: CreateUserRequest) => client.post<UserInfo>(`${V1}/users`, u),
      update: (id: string, u: UpdateUserRequest) => client.patch<UserInfo>(`${V1}/users/${seg(id)}`, u),
    },
    audit: {
      list: (q: AuditQuery = {}) => client.get<Page<AuditEvent>>(`${V1}/audit-events`, { query: q as Query }),
    },
  }
}

export type Endpoints = ReturnType<typeof createEndpoints>
