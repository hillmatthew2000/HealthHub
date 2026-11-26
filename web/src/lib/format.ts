import type { CodeableConcept, HumanName, Observation, Patient, UserInfo } from '@/api/types'

export function formatName(n: HumanName | undefined): string {
  if (!n) return ''
  return [...(n.prefix ?? []), ...(n.given ?? []), n.family, ...(n.suffix ?? [])].filter(Boolean).join(' ')
}

export function patientName(p: Pick<Patient, 'name'> | undefined): string {
  return formatName(p?.name?.[0]) || 'Unnamed patient'
}

export function userName(u: Pick<UserInfo, 'firstName' | 'lastName' | 'email'> | undefined): string {
  if (!u) return ''
  return `${u.firstName} ${u.lastName}`.trim() || u.email
}

export function initials(text: string): string {
  const parts = text.trim().split(/\s+/).filter(Boolean)
  const first = parts[0]?.[0] ?? ''
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? '') : ''
  return (first + last).toUpperCase() || '?'
}

const dateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const dateTimeFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

/** Formats a YYYY-MM-DD date without shifting it through the local time zone. */
export function formatDate(value: string | undefined): string {
  if (!value) return ''
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (m) return dateFmt.format(new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])))
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? value : dateFmt.format(d)
}

export function formatDateTime(value: string | undefined): string {
  if (!value) return ''
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? value : dateTimeFmt.format(d)
}

/** Age in whole years from a YYYY-MM-DD birth date. */
export function age(birthDate: string | undefined, now = new Date()): number | undefined {
  const m = birthDate && /^(\d{4})-(\d{2})-(\d{2})$/.exec(birthDate)
  if (!m) return undefined
  const y = Number(m[1])
  const mo = Number(m[2]) - 1
  const d = Number(m[3])
  let years = now.getFullYear() - y
  if (now.getMonth() < mo || (now.getMonth() === mo && now.getDate() < d)) years--
  return years >= 0 ? years : undefined
}

export function conceptText(cc: CodeableConcept | undefined): string {
  if (!cc) return ''
  const c = cc.coding?.find((x) => x.display || x.code)
  return cc.text || c?.display || c?.code || ''
}

export function observationTitle(o: Observation): string {
  return conceptText(o.code) || 'Unknown'
}

export function observationValue(o: Observation): string {
  const q = o.valueQuantity
  if (q && q.value !== undefined) {
    const unit = q.unit || q.code || ''
    return `${q.comparator ?? ''}${q.value}${unit ? ` ${unit}` : ''}`
  }
  if (o.valueString) return o.valueString
  if (typeof o.valueBoolean === 'boolean') return o.valueBoolean ? 'Yes' : 'No'
  if (typeof o.valueInteger === 'number') return String(o.valueInteger)
  if (o.valueCodeableConcept) return conceptText(o.valueCodeableConcept)
  return ''
}

const abnormalCodes = new Set(['A', 'AA', 'HH', 'LL', 'H', 'L'])

/** Matches Observation.IsAbnormal in the Go models. */
export function isAbnormal(o: Observation): boolean {
  return (o.interpretation ?? []).some((i) => (i.coding ?? []).some((c) => c.code && abnormalCodes.has(c.code)))
}

export function observationPatientId(o: Pick<Observation, 'subject'>): string | undefined {
  const ref = o.subject?.reference
  return ref?.startsWith('Patient/') ? ref.slice('Patient/'.length) || undefined : undefined
}

export function categoryCodes(o: Observation): string[] {
  return (o.category ?? []).flatMap((c) => c.coding.map((x) => x.display || x.code || '').filter(Boolean))
}

export function shortId(id: string | undefined): string {
  return id ? id.slice(0, 8) : ''
}

export function titleCase(s: string): string {
  return s.replace(/(^|[-\s])(\w)/g, (_m, sep: string, ch: string) => (sep === '-' ? ' ' : sep) + ch.toUpperCase())
}
