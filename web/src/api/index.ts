import { readRuntimeConfig } from '@/config'
import { ApiClient } from './client'
import { createEndpoints } from './endpoints'
import type { Problem } from './types'

export * from './client'
export * from './types'

type TokenProvider = () => string | null
type UnauthorizedHandler = (problem: Problem) => void

let tokenProvider: TokenProvider = () => null
let unauthorizedHandler: UnauthorizedHandler | undefined

/** Wires the shared client to the session. Called once by the auth store. */
export function configureClientAuth(getToken: TokenProvider, onUnauthorized: UnauthorizedHandler): void {
  tokenProvider = getToken
  unauthorizedHandler = onUnauthorized
}

export const client = new ApiClient({
  baseUrl: () => readRuntimeConfig().apiBaseUrl,
  getToken: () => tokenProvider(),
  onUnauthorized: (p) => unauthorizedHandler?.(p),
})

export const api = createEndpoints(client)
