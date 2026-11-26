import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { client } from '@/api'
import type { AuthResponse } from '@/api/types'
import { IDLE_LIMIT_MS, REFRESH_LEAD_MS, SESSION_KEY, useAuthStore } from './auth'

const HOUR = 60 * 60_000

function authResponse(token: string, ttlMs = HOUR, roles: AuthResponse['user']['roles'] = ['nurse']): AuthResponse {
  return {
    token,
    expiresAt: new Date(Date.now() + ttlMs).toISOString(),
    user: { id: 'u1', email: 'n@example.org', firstName: 'Nora', lastName: 'Nurse', roles, active: true, createdAt: '2026-01-01T00:00:00Z' },
  }
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-11-26T12:00:00Z'))
  sessionStorage.clear()
  fetchMock = vi.fn<typeof fetch>()
  vi.stubGlobal('fetch', fetchMock)
  setActivePinia(createPinia())
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('auth store', () => {
  it('logs in, keeps the token in memory and sessionStorage, and exposes role checks', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'correct horse battery')
    expect(auth.token).toBe('t1')
    expect(auth.isAuthenticated).toBe(true)
    expect(JSON.parse(sessionStorage.getItem(SESSION_KEY)!)).toMatchObject({ token: 't1', user: { id: 'u1' } })
    expect(auth.can('patients', 'read')).toBe(true)
    expect(auth.can('patients', 'create')).toBe(false)
    expect(auth.isAdmin).toBe(false)
  })

  it('attaches the token to later requests', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')
    fetchMock.mockResolvedValueOnce(json(200, { data: [], page: 1, limit: 20, total: 0, totalPages: 0 }))
    await client.get('/api/v1/patients')
    const headers = fetchMock.mock.calls[1]![1]?.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer t1')
  })

  it('restores an unexpired session and discards an expired one', () => {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify({ token: 'saved', expiresAt: Date.now() + HOUR, user: authResponse('x').user }))
    const auth = useAuthStore()
    auth.restore()
    expect(auth.token).toBe('saved')

    setActivePinia(createPinia())
    sessionStorage.setItem(SESSION_KEY, JSON.stringify({ token: 'old', expiresAt: Date.now() - 1, user: authResponse('x').user }))
    const auth2 = useAuthStore()
    auth2.restore()
    expect(auth2.token).toBeNull()
    expect(sessionStorage.getItem(SESSION_KEY)).toBeNull()
  })

  it('ignores corrupt stored sessions', () => {
    sessionStorage.setItem(SESSION_KEY, '{not json')
    const auth = useAuthStore()
    auth.restore()
    expect(auth.token).toBeNull()
  })

  it('refreshes silently before the token expires', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')

    fetchMock.mockResolvedValueOnce(json(200, authResponse('t2')))
    await vi.advanceTimersByTimeAsync(HOUR - REFRESH_LEAD_MS - 1000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    auth.markActivity()
    await vi.advanceTimersByTimeAsync(2000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    const [url, init] = fetchMock.mock.calls[1]!
    expect(url).toBe('/api/v1/auth/refresh')
    expect(init?.method).toBe('POST')
    expect((init?.headers as Record<string, string>).Authorization).toBe('Bearer t1')
    expect(auth.token).toBe('t2')
    expect(JSON.parse(sessionStorage.getItem(SESSION_KEY)!).token).toBe('t2')
  })

  it('does not refresh an idle session and logs out when it expires', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1', IDLE_LIMIT_MS + 10 * 60_000)))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')
    await vi.advanceTimersByTimeAsync(IDLE_LIMIT_MS + 10 * 60_000 + 1)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(auth.token).toBeNull()
    expect(auth.logoutReason).toBe('expired')
  })

  it('logs out when the API rejects the token with 401', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')
    fetchMock.mockResolvedValueOnce(json(401, { status: 401, title: 'Unauthorized', code: 'invalid_token' }))
    await expect(client.get('/api/v1/patients')).rejects.toMatchObject({ status: 401 })
    expect(auth.token).toBeNull()
    expect(auth.logoutReason).toBe('unauthorized')
    expect(sessionStorage.getItem(SESSION_KEY)).toBeNull()
  })

  it('retries a refresh that failed for a transient reason', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')
    fetchMock.mockResolvedValueOnce(json(503, { status: 503, code: 'service_unavailable' }))
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t2')))
    await vi.advanceTimersByTimeAsync(HOUR - REFRESH_LEAD_MS - 1000)
    auth.markActivity()
    await vi.advanceTimersByTimeAsync(2000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(auth.token).toBe('t1')
    await vi.advanceTimersByTimeAsync(30_000)
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(auth.token).toBe('t2')
  })

  it('shares one request between concurrent refreshes', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t2')))
    await Promise.all([auth.refresh(), auth.refresh()])
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('clears everything on logout and stops timers', async () => {
    fetchMock.mockResolvedValueOnce(json(200, authResponse('t1')))
    const auth = useAuthStore()
    await auth.login('n@example.org', 'pw')
    auth.logout('manual')
    expect(auth.token).toBeNull()
    expect(auth.user).toBeNull()
    expect(auth.logoutReason).toBe('manual')
    expect(sessionStorage.getItem(SESSION_KEY)).toBeNull()
    await vi.advanceTimersByTimeAsync(2 * HOUR)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
