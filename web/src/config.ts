/**
 * Runtime configuration, read from window.__HEALTHHUB_CONFIG__ which
 * /config.js sets before the app loads. The container writes that file from
 * environment variables, so one build serves every environment.
 */
export interface RuntimeConfig {
  /** API origin plus optional path prefix, without a trailing slash. '' is same origin. */
  apiBaseUrl: string
}

export function normalizeBaseUrl(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value.trim().replace(/\/+$/, '')
}

export function readRuntimeConfig(
  source: HealthHubRuntimeConfig | undefined = typeof window === 'undefined' ? undefined : window.__HEALTHHUB_CONFIG__,
): RuntimeConfig {
  return { apiBaseUrl: normalizeBaseUrl(source?.apiBaseUrl) }
}
