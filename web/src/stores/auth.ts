import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, ApiError, configureClientAuth } from '@/api'
import type { AuthResponse, UserInfo } from '@/api/types'
import { can as canDo, type Action, type Resource } from '@/lib/permissions'
import { clearPatientNames } from '@/lib/patientNames'

/** sessionStorage key. The session lives only as long as the browser tab. */
export const SESSION_KEY = 'healthhub.session'
/** Refresh this long before the token expires. */
export const REFRESH_LEAD_MS = 5 * 60_000
/** Never schedule a refresh sooner than this. */
export const MIN_REFRESH_DELAY_MS = 10_000
/** Retry a failed refresh (network error, 5xx) after this delay. */
export const REFRESH_RETRY_MS = 30_000
/**
 * Stop refreshing after this much inactivity, so an unattended screen with
 * patient data on it times out when the token expires.
 */
export const IDLE_LIMIT_MS = 30 * 60_000

export type LogoutReason = 'manual' | 'expired' | 'idle' | 'unauthorized'

interface StoredSession {
  token: string
  expiresAt: number
  user: UserInfo
}

function readStored(): StoredSession | null {
  try {
    const raw = sessionStorage.getItem(SESSION_KEY)
    if (!raw) return null
    const s = JSON.parse(raw) as Partial<StoredSession>
    if (typeof s.token !== 'string' || typeof s.expiresAt !== 'number' || !s.user) return null
    return s as StoredSession
  } catch {
    return null
  }
}

function writeStored(s: StoredSession | null): void {
  try {
    if (s) sessionStorage.setItem(SESSION_KEY, JSON.stringify(s))
    else sessionStorage.removeItem(SESSION_KEY)
  } catch {
    // Storage can be unavailable (private mode, blocked site data). The
    // session then lasts until the page is reloaded.
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const expiresAt = ref<number | null>(null)
  const user = ref<UserInfo | null>(null)
  const lastActivity = ref(Date.now())
  const logoutReason = ref<LogoutReason | null>(null)

  let refreshTimer: ReturnType<typeof setTimeout> | undefined
  let expiryTimer: ReturnType<typeof setTimeout> | undefined
  let refreshing: Promise<void> | null = null

  const isAuthenticated = computed(() => !!token.value && !!expiresAt.value && expiresAt.value > Date.now())
  const roles = computed(() => user.value?.roles ?? [])
  const isAdmin = computed(() => roles.value.includes('admin'))

  function can(resource: Resource, action: Action): boolean {
    return canDo(roles.value, resource, action)
  }

  function clearTimers(): void {
    clearTimeout(refreshTimer)
    clearTimeout(expiryTimer)
    refreshTimer = expiryTimer = undefined
  }

  function schedule(): void {
    clearTimers()
    if (!expiresAt.value) return
    const remaining = expiresAt.value - Date.now()
    if (remaining <= 0) {
      logout('expired')
      return
    }
    expiryTimer = setTimeout(() => logout('expired'), remaining)
    const delay = Math.max(remaining - REFRESH_LEAD_MS, Math.min(MIN_REFRESH_DELAY_MS, remaining / 2))
    refreshTimer = setTimeout(() => void silentRefresh(), delay)
  }

  function applySession(resp: AuthResponse): void {
    const exp = Date.parse(resp.expiresAt)
    token.value = resp.token
    expiresAt.value = Number.isNaN(exp) ? Date.now() + 60 * 60_000 : exp
    user.value = resp.user
    logoutReason.value = null
    writeStored({ token: resp.token, expiresAt: expiresAt.value, user: resp.user })
    schedule()
  }

  /** Restores a session saved in this tab, discarding it if it has expired. */
  function restore(): void {
    const s = readStored()
    if (!s || s.expiresAt <= Date.now()) {
      writeStored(null)
      return
    }
    token.value = s.token
    expiresAt.value = s.expiresAt
    user.value = s.user
    lastActivity.value = Date.now()
    schedule()
  }

  async function login(email: string, password: string): Promise<void> {
    const resp = await api.auth.login({ email, password })
    lastActivity.value = Date.now()
    applySession(resp)
  }

  /** Exchanges the current token for a new one. Concurrent calls share one request. */
  function refresh(): Promise<void> {
    if (!token.value) return Promise.resolve()
    refreshing ??= api.auth
      .refresh()
      .then(applySession)
      .finally(() => {
        refreshing = null
      })
    return refreshing
  }

  async function silentRefresh(): Promise<void> {
    if (!token.value) return
    if (Date.now() - lastActivity.value > IDLE_LIMIT_MS) {
      // Let the token run out; the expiry timer logs the user out.
      return
    }
    try {
      await refresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        logout('unauthorized')
        return
      }
      // Transient failure: try again while the token is still valid.
      const remaining = (expiresAt.value ?? 0) - Date.now()
      if (remaining > REFRESH_RETRY_MS) {
        clearTimeout(refreshTimer)
        refreshTimer = setTimeout(() => void silentRefresh(), REFRESH_RETRY_MS)
      }
    }
  }

  /** Updates the cached user, for example after loading the profile. */
  function setUser(u: UserInfo): void {
    user.value = u
    if (token.value && expiresAt.value) writeStored({ token: token.value, expiresAt: expiresAt.value, user: u })
  }

  function markActivity(): void {
    lastActivity.value = Date.now()
  }

  function logout(reason: LogoutReason = 'manual'): void {
    clearTimers()
    if (token.value) logoutReason.value = reason
    token.value = null
    expiresAt.value = null
    user.value = null
    writeStored(null)
    clearPatientNames()
  }

  configureClientAuth(
    () => token.value,
    () => {
      if (token.value) logout('unauthorized')
    },
  )

  return {
    token,
    expiresAt,
    user,
    roles,
    isAdmin,
    isAuthenticated,
    logoutReason,
    lastActivity,
    can,
    restore,
    login,
    refresh,
    silentRefresh,
    setUser,
    markActivity,
    logout,
  }
})
