import type { FieldError, Problem } from './types'

/**
 * An error response from the API. It carries the RFC 9457 problem details
 * the API returns, or a synthesized problem when the response had none
 * (network failures, proxies answering with HTML and so on).
 */
export class ApiError extends Error {
  readonly status: number
  readonly problem: Problem

  constructor(problem: Problem) {
    super(problem.detail || problem.title || `Request failed with status ${problem.status}`)
    this.name = 'ApiError'
    this.status = problem.status
    this.problem = problem
  }

  get code(): string {
    return this.problem.code
  }

  /** Field errors keyed by field path, for example "name[0].family". */
  get fieldErrors(): Record<string, string> {
    const out: Record<string, string> = {}
    for (const e of this.problem.errors ?? []) {
      if (!(e.field in out)) out[e.field] = e.message
    }
    return out
  }
}

/** The problem code for requests that never got an HTTP response (status 0). */
export const NETWORK_ERROR = 'network_error'

const statusTitles: Record<number, string> = {
  400: 'Bad Request',
  401: 'Unauthorized',
  403: 'Forbidden',
  404: 'Not Found',
  409: 'Conflict',
  413: 'Content Too Large',
  422: 'Unprocessable Content',
  429: 'Too Many Requests',
  500: 'Internal Server Error',
  502: 'Bad Gateway',
  503: 'Service Unavailable',
  504: 'Gateway Timeout',
}

function isProblemLike(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function asFieldErrors(value: unknown): FieldError[] | undefined {
  if (!Array.isArray(value)) return undefined
  return value.filter(
    (e): e is FieldError =>
      typeof e === 'object' && e !== null && typeof e.field === 'string' && typeof e.message === 'string',
  )
}

function str(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}

/** Builds a problem from a response body, falling back to the status line. */
export function toProblem(status: number, body: unknown, statusText = ''): Problem {
  const fallbackTitle = statusTitles[status] || statusText || 'Error'
  if (isProblemLike(body) && (typeof body.code === 'string' || typeof body.detail === 'string' || typeof body.title === 'string')) {
    return {
      type: str(body.type) ?? 'about:blank',
      title: str(body.title) ?? fallbackTitle,
      status,
      detail: str(body.detail),
      instance: str(body.instance),
      code: str(body.code) ?? `http_${status}`,
      requestId: str(body.requestId),
      errors: asFieldErrors(body.errors),
    }
  }
  return {
    type: 'about:blank',
    title: fallbackTitle,
    status,
    code: `http_${status}`,
    detail: status === 429 ? 'Too many requests. Wait a moment and try again.' : undefined,
  }
}

/** A user-facing message for any error thrown by the client. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    const p = err.problem
    if (p.errors?.length) {
      return `${p.detail || p.title} ${p.errors.map((e) => `${e.field} ${e.message}.`).join(' ')}`
    }
    return p.detail || p.title
  }
  if (err instanceof Error) return err.message
  return 'Something went wrong.'
}

export type QueryValue = string | number | boolean | undefined | null
export type Query = Record<string, QueryValue>

export interface RequestOptions {
  query?: Query
  body?: unknown
  signal?: AbortSignal
  /** Send the bearer token and treat 401 as an expired session. Default true. */
  auth?: boolean
}

export interface ClientOptions {
  /** Returns the API base URL; read per request so runtime config changes apply. */
  baseUrl: () => string
  /** Returns the current bearer token, if any. */
  getToken: () => string | null
  /** Called when an authenticated request is rejected with 401. */
  onUnauthorized?: (problem: Problem) => void
  fetch?: typeof fetch
}

export function buildUrl(baseUrl: string, path: string, query?: Query): string {
  const base = baseUrl.replace(/\/+$/, '')
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value === undefined || value === null || value === '') continue
    params.append(key, String(value))
  }
  const qs = params.toString()
  return `${base}${path.startsWith('/') ? path : `/${path}`}${qs ? `?${qs}` : ''}`
}

async function readBody(res: Response): Promise<unknown> {
  if (res.status === 204) return undefined
  const type = res.headers.get('Content-Type') ?? ''
  const text = await res.text()
  if (!text || !type.includes('json')) return undefined
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

export class ApiClient {
  private readonly opts: ClientOptions

  constructor(opts: ClientOptions) {
    this.opts = opts
  }

  /** Sends a request and returns the response and parsed body without throwing on HTTP errors. */
  async send(method: string, path: string, options: RequestOptions = {}): Promise<{ response: Response; body: unknown }> {
    const useAuth = options.auth ?? true
    const headers: Record<string, string> = { Accept: 'application/json, application/problem+json' }
    let payload: string | undefined
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json'
      payload = JSON.stringify(options.body)
    }
    if (useAuth) {
      const token = this.opts.getToken()
      if (token) headers.Authorization = `Bearer ${token}`
    }
    const url = buildUrl(this.opts.baseUrl(), path, options.query)
    const doFetch = this.opts.fetch ?? globalThis.fetch.bind(globalThis)
    let response: Response
    try {
      // Bearer tokens only: never send cookies to the API.
      response = await doFetch(url, { method, headers, body: payload, signal: options.signal, credentials: 'omit' })
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') throw err
      throw new ApiError({
        type: 'about:blank',
        title: 'Network error',
        status: 0,
        code: NETWORK_ERROR,
        detail: 'Could not reach the HealthHub API. Check your connection and the configured API address.',
      })
    }
    const body = await readBody(response)
    return { response, body }
  }

  async request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
    const { response, body } = await this.send(method, path, options)
    if (response.ok) return body as T
    const problem = toProblem(response.status, body, response.statusText)
    if (response.status === 401 && (options.auth ?? true)) {
      this.opts.onUnauthorized?.(problem)
    }
    throw new ApiError(problem)
  }

  get<T>(path: string, options?: Omit<RequestOptions, 'body'>): Promise<T> {
    return this.request<T>('GET', path, options)
  }

  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>('POST', path, { ...options, body })
  }

  put<T>(path: string, body: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>('PUT', path, { ...options, body })
  }

  patch<T>(path: string, body: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>('PATCH', path, { ...options, body })
  }

  delete(path: string, options?: RequestOptions): Promise<void> {
    return this.request<void>('DELETE', path, options)
  }
}
