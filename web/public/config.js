// Runtime configuration. The production container overwrites this file at
// start-up from the API_BASE_URL environment variable. An empty apiBaseUrl
// means the API is served from the same origin as this app.
window.__HEALTHHUB_CONFIG__ = { apiBaseUrl: '' }
