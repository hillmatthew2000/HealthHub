// Client-side checks that mirror the API's validation rules (the validate
// struct tags in internal/models). The API stays authoritative; these give
// immediate feedback and use the same field paths, so API field errors land
// on the same inputs.

export type Errors = Record<string, string>

/** Length in Unicode code points, which is how the Go validator counts. */
export function charLength(s: string): number {
  return [...s].length
}

export function byteLength(s: string): number {
  return new TextEncoder().encode(s).length
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function isEmail(s: string): boolean {
  return EMAIL_RE.test(s.trim())
}

export const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function isUuid(s: string): boolean {
  return UUID_RE.test(s.trim())
}

/** Accepts a real calendar date in YYYY-MM-DD form. */
export function isIsoDate(s: string): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s)
  if (!m) return false
  const d = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3])))
  return d.getUTCFullYear() === Number(m[1]) && d.getUTCMonth() === Number(m[2]) - 1 && d.getUTCDate() === Number(m[3])
}

export function maxChars(errors: Errors, field: string, value: string | undefined, max: number): void {
  if (value && charLength(value) > max) errors[field] = `must have at most ${max} characters`
}

export function required(errors: Errors, field: string, value: string | undefined): boolean {
  if (!value || !value.trim()) {
    errors[field] = 'is required'
    return false
  }
  return true
}

export const PASSWORD_MIN = 12
export const PASSWORD_MAX = 72

/**
 * New passwords must be 12 to 72 characters (the request validator) and at
 * most 72 bytes in UTF-8 (bcrypt, enforced by the password hasher).
 */
export function passwordProblem(pw: string): string | undefined {
  if (!pw) return 'is required'
  const n = charLength(pw)
  if (n < PASSWORD_MIN) return `must have at least ${PASSWORD_MIN} characters`
  if (n > PASSWORD_MAX) return `must have at most ${PASSWORD_MAX} characters`
  if (byteLength(pw) > PASSWORD_MAX) return `must be at most ${PASSWORD_MAX} bytes; use fewer non-ASCII characters`
  return undefined
}

/**
 * Looks up the error for a field, also matching errors reported on its
 * children (for example "name[0].given[1]" for "name[0].given").
 */
export function fieldError(errors: Errors, field: string): string | undefined {
  if (errors[field]) return errors[field]
  for (const [k, v] of Object.entries(errors)) {
    if (k.startsWith(`${field}[`) || k.startsWith(`${field}.`)) return v
  }
  return undefined
}

export function hasErrors(errors: Errors): boolean {
  return Object.keys(errors).length > 0
}
