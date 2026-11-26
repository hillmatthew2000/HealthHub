import type { LocationQuery } from 'vue-router'

/** Reads a single string from a route query value. */
export function qs(q: LocationQuery, key: string): string {
  const v = q[key]
  const s = Array.isArray(v) ? v[0] : v
  return typeof s === 'string' ? s : ''
}

export function qint(q: LocationQuery, key: string, def: number, min = 1, max = Number.MAX_SAFE_INTEGER): number {
  const n = Number.parseInt(qs(q, key), 10)
  return Number.isFinite(n) && n >= min && n <= max ? n : def
}

/** Drops empty values so URLs stay short. */
export function compactQuery(q: Record<string, string | number | boolean | undefined>): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(q)) {
    if (v === undefined || v === '' || v === null) continue
    out[k] = String(v)
  }
  return out
}
