/// <reference types="vite/client" />

interface HealthHubRuntimeConfig {
  /** Origin (and optional path prefix) of the HealthHub API. Empty means same origin. */
  apiBaseUrl?: string
}

interface Window {
  __HEALTHHUB_CONFIG__?: HealthHubRuntimeConfig
}
