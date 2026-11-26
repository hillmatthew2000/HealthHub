import { describe, expect, it, vi } from 'vitest'
import { ApiClient, ApiError, NETWORK_ERROR, buildUrl, errorMessage, toProblem } from './client'
import { createEndpoints } from './endpoints'

function json(status: number, body: unknown, type = 'application/json'): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': type } })
}

function setup(token: string | null = 'tok', baseUrl = 'http://api.test') {
  const fetchMock = vi.fn<typeof fetch>()
  const onUnauthorized = vi.fn()
  const client = new ApiClient({ baseUrl: () => baseUrl, getToken: () => token, onUnauthorized, fetch: fetchMock })
  return { client, fetchMock, onUnauthorized, api: createEndpoints(client) }
}

const problem = {
  type: 'about:blank',
  title: 'Unprocessable Entity',
  status: 422,
  detail: 'One or more fields are invalid.',
  instance: '/api/v1/patients',
  code: 'validation_failed',
  requestId: 'req-1',
  errors: [
    { field: 'name[0].family', message: 'is required' },
    { field: 'gender', message: 'must be one of: male female other unknown' },
  ],
}

describe('buildUrl', () => {
  it('joins the base URL and path and drops empty query values', () => {
    expect(buildUrl('http://api.test/', '/api/v1/patients', { page: 2, family: '', gender: undefined, active: false })).toBe(
      'http://api.test/api/v1/patients?page=2&active=false',
    )
  })

  it('uses same-origin paths when the base URL is empty', () => {
    expect(buildUrl('', '/health/ready')).toBe('/health/ready')
  })

  it('encodes query values', () => {
    expect(buildUrl('', '/x', { code: 'http://loinc.org|8867-4' })).toBe('/x?code=http%3A%2F%2Floinc.org%7C8867-4')
  })
})

describe('ApiClient', () => {
  it('sends the bearer token and JSON body', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(201, { id: 'p1' }))
    await api.patients.create({ active: true, gender: 'female', name: [{ family: 'Doe', given: ['Jane'] }] })
    const [url, init] = fetchMock.mock.calls[0]!
    expect(url).toBe('http://api.test/api/v1/patients')
    expect(init?.method).toBe('POST')
    const headers = init?.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer tok')
    expect(headers['Content-Type']).toBe('application/json')
    expect(init?.credentials).toBe('omit')
    expect(JSON.parse(String(init?.body))).toMatchObject({ gender: 'female' })
  })

  it('does not send the token to public endpoints', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(200, { token: 't', expiresAt: '2030-01-01T00:00:00Z', user: {} }))
    await api.auth.login({ email: 'a@b.c', password: 'x' })
    const headers = fetchMock.mock.calls[0]![1]?.headers as Record<string, string>
    expect(headers.Authorization).toBeUndefined()
  })

  it('surfaces RFC 9457 problem details', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(422, problem, 'application/problem+json'))
    const err = await api.patients.create({ active: true, gender: 'male', name: [] }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    const e = err as ApiError
    expect(e.status).toBe(422)
    expect(e.code).toBe('validation_failed')
    expect(e.message).toBe('One or more fields are invalid.')
    expect(e.problem.requestId).toBe('req-1')
    expect(e.fieldErrors).toEqual({
      'name[0].family': 'is required',
      gender: 'must be one of: male female other unknown',
    })
    expect(errorMessage(e)).toContain('name[0].family is required.')
  })

  it('calls onUnauthorized for a 401 on an authenticated request', async () => {
    const { api, fetchMock, onUnauthorized } = setup()
    fetchMock.mockResolvedValue(
      json(401, { title: 'Unauthorized', status: 401, code: 'invalid_token', detail: 'The access token is invalid or has expired.' }, 'application/problem+json'),
    )
    await expect(api.patients.list()).rejects.toMatchObject({ status: 401, code: 'invalid_token' })
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('does not treat failed logins as an expired session', async () => {
    const { api, fetchMock, onUnauthorized } = setup(null)
    fetchMock.mockResolvedValue(
      json(401, { title: 'Unauthorized', status: 401, code: 'invalid_credentials', detail: 'The email address or password is incorrect.' }),
    )
    await expect(api.auth.login({ email: 'a@b.c', password: 'nope' })).rejects.toMatchObject({ code: 'invalid_credentials' })
    expect(onUnauthorized).not.toHaveBeenCalled()
  })

  it('returns undefined for 204 responses', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }))
    await expect(api.patients.remove('11111111-1111-4111-8111-111111111111')).resolves.toBeUndefined()
    expect(fetchMock.mock.calls[0]![1]?.method).toBe('DELETE')
  })

  it('synthesizes a problem for non-JSON errors', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(new Response('<html>Bad gateway</html>', { status: 502, headers: { 'Content-Type': 'text/html' } }))
    await expect(api.patients.list()).rejects.toMatchObject({ status: 502, code: 'http_502', message: 'Bad Gateway' })
  })

  it('reports network failures with status 0', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'))
    await expect(api.patients.list()).rejects.toMatchObject({ status: 0, code: NETWORK_ERROR })
  })

  it('encodes path IDs and sends query filters', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(200, { data: [], page: 1, limit: 20, total: 0, totalPages: 0 }))
    await api.patients.observations('a/b', { status: 'final', from: '2026-01-01' })
    expect(fetchMock.mock.calls[0]![0]).toBe('http://api.test/api/v1/patients/a%2Fb/observations?status=final&from=2026-01-01')
  })

  it('uses PATCH for user updates', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(200, {}))
    await api.users.update('u1', { active: false })
    expect(fetchMock.mock.calls[0]![1]?.method).toBe('PATCH')
    expect(fetchMock.mock.calls[0]![1]?.body).toBe('{"active":false}')
  })
})

describe('health', () => {
  it('reads the readiness probe at the root, not under /api/v1', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(200, { status: 'ready', version: '1.2.3', checks: { database: 'up' } }))
    await expect(api.health.ready()).resolves.toEqual({ ok: true, status: 'ready', version: '1.2.3', checks: { database: 'up' } })
    expect(fetchMock.mock.calls[0]![0]).toBe('http://api.test/health/ready')
  })

  it('reports a 503 as not ok without throwing', async () => {
    const { api, fetchMock } = setup()
    fetchMock.mockResolvedValue(json(503, { status: 'not ready', checks: { database: 'down' } }))
    await expect(api.health.ready()).resolves.toMatchObject({ ok: false, status: 'not ready' })
  })
})

describe('toProblem', () => {
  it('falls back to the status title', () => {
    expect(toProblem(429, undefined)).toMatchObject({ title: 'Too Many Requests', code: 'http_429' })
  })

  it('ignores malformed field errors', () => {
    const p = toProblem(422, { code: 'validation_failed', errors: [{ field: 'a', message: 'b' }, { nope: 1 }, 'x'] })
    expect(p.errors).toEqual([{ field: 'a', message: 'b' }])
  })
})
