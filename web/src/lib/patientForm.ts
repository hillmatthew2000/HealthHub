import type { Address, ContactPoint, Gender, HumanName, Patient, PatientInput, PatientQuery } from '@/api/types'
import { ADDRESS_TYPES, ADDRESS_USES, GENDERS, NAME_USES, TELECOM_SYSTEMS, TELECOM_USES } from '@/api/types'
import { type Errors, isIsoDate, maxChars, required } from './validation'

export interface TelecomRow {
  system: string
  value: string
  use: string
  rank: string
}

export interface AddressRow {
  use: string
  type: string
  lines: string
  city: string
  district: string
  state: string
  postalCode: string
  country: string
  text: string
  /** Fields the form does not edit, kept as they were. */
  period?: Address['period']
}

export interface PatientForm {
  active: boolean
  gender: Gender | ''
  birthDate: string
  nameUse: string
  family: string
  given: string
  prefix: string
  suffix: string
  /** Names after the first; the form shows but does not edit them. */
  otherNames: HumanName[]
  telecom: TelecomRow[]
  address: AddressRow[]
}

const words = (s: string) => s.split(/\s+/).filter(Boolean)
const lines = (s: string) =>
  s
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)

export function emptyTelecom(): TelecomRow {
  return { system: 'phone', value: '', use: '', rank: '' }
}

export function emptyAddress(): AddressRow {
  return { use: 'home', type: '', lines: '', city: '', district: '', state: '', postalCode: '', country: '', text: '' }
}

export function emptyPatientForm(): PatientForm {
  return {
    active: true,
    gender: '',
    birthDate: '',
    nameUse: 'official',
    family: '',
    given: '',
    prefix: '',
    suffix: '',
    otherNames: [],
    telecom: [],
    address: [],
  }
}

export function patientToForm(p: Patient): PatientForm {
  const [first, ...rest] = p.name ?? []
  return {
    active: p.active,
    gender: p.gender,
    birthDate: p.birthDate ?? '',
    nameUse: first?.use ?? '',
    family: first?.family ?? '',
    given: (first?.given ?? []).join(' '),
    prefix: (first?.prefix ?? []).join(' '),
    suffix: (first?.suffix ?? []).join(' '),
    otherNames: rest,
    telecom: (p.telecom ?? []).map((t) => ({
      system: t.system,
      value: t.value,
      use: t.use ?? '',
      rank: t.rank ? String(t.rank) : '',
    })),
    address: (p.address ?? []).map((a) => ({
      use: a.use ?? '',
      type: a.type ?? '',
      lines: (a.line ?? []).join('\n'),
      city: a.city ?? '',
      district: a.district ?? '',
      state: a.state ?? '',
      postalCode: a.postalCode ?? '',
      country: a.country ?? '',
      text: a.text ?? '',
      period: a.period,
    })),
  }
}

function opt(s: string): string | undefined {
  const t = s.trim()
  return t === '' ? undefined : t
}

export function formToPatientInput(f: PatientForm): PatientInput {
  const name: HumanName = {
    use: opt(f.nameUse),
    family: f.family.trim(),
    given: words(f.given),
  }
  const prefix = words(f.prefix)
  const suffix = words(f.suffix)
  if (prefix.length) name.prefix = prefix
  if (suffix.length) name.suffix = suffix

  const telecom: ContactPoint[] = f.telecom.map((t) => {
    const c: ContactPoint = { system: t.system, value: t.value.trim() }
    if (t.use) c.use = t.use
    if (t.rank.trim()) c.rank = Number(t.rank)
    return c
  })
  const address: Address[] = f.address.map((a) => {
    const out: Address = {
      use: opt(a.use),
      type: opt(a.type),
      line: lines(a.lines),
      city: opt(a.city),
      district: opt(a.district),
      state: opt(a.state),
      postalCode: opt(a.postalCode),
      country: opt(a.country),
      text: opt(a.text),
      period: a.period,
    }
    if (!out.line?.length) delete out.line
    for (const k of Object.keys(out) as (keyof Address)[]) if (out[k] === undefined) delete out[k]
    return out
  })

  const input: PatientInput = {
    active: f.active,
    gender: f.gender as Gender,
    name: [name, ...f.otherNames],
  }
  if (f.birthDate) input.birthDate = f.birthDate
  if (telecom.length) input.telecom = telecom
  if (address.length) input.address = address
  return input
}

/** Validates the form against the rules in internal/models/patient.go. */
export function validatePatientForm(f: PatientForm, today = new Date()): Errors {
  const e: Errors = {}
  if (!f.gender) e.gender = 'is required'
  else if (!GENDERS.includes(f.gender)) e.gender = `must be one of: ${GENDERS.join(' ')}`

  if (f.birthDate) {
    if (!isIsoDate(f.birthDate)) e.birthDate = 'must be a YYYY-MM-DD date'
    else if (f.birthDate > today.toISOString().slice(0, 10)) e.birthDate = 'cannot be in the future'
  }

  if (f.nameUse && !(NAME_USES as readonly string[]).includes(f.nameUse)) e['name[0].use'] = 'is not a valid name use'
  if (required(e, 'name[0].family', f.family)) maxChars(e, 'name[0].family', f.family, 200)
  const given = words(f.given)
  if (given.length === 0) e['name[0].given'] = 'is required'
  else if (given.length > 10) e['name[0].given'] = 'must have at most 10 items'
  else given.forEach((g, i) => maxChars(e, `name[0].given[${i}]`, g, 200))
  for (const key of ['prefix', 'suffix'] as const) {
    const parts = words(f[key])
    if (parts.length > 5) e[`name[0].${key}`] = 'must have at most 5 items'
    else parts.forEach((p, i) => maxChars(e, `name[0].${key}[${i}]`, p, 50))
  }
  if (1 + f.otherNames.length > 10) e.name = 'must have at most 10 items'

  if (f.telecom.length > 20) e.telecom = 'must have at most 20 items'
  f.telecom.forEach((t, i) => {
    const p = `telecom[${i}]`
    if (!(TELECOM_SYSTEMS as readonly string[]).includes(t.system)) e[`${p}.system`] = 'is required'
    if (required(e, `${p}.value`, t.value)) maxChars(e, `${p}.value`, t.value.trim(), 254)
    if (t.use && !(TELECOM_USES as readonly string[]).includes(t.use)) e[`${p}.use`] = 'is not a valid use'
    if (t.rank.trim() && !(/^\d+$/.test(t.rank.trim()))) e[`${p}.rank`] = 'must be a whole number of 0 or more'
  })

  if (f.address.length > 10) e.address = 'must have at most 10 items'
  f.address.forEach((a, i) => {
    const p = `address[${i}]`
    if (a.use && !(ADDRESS_USES as readonly string[]).includes(a.use)) e[`${p}.use`] = 'is not a valid use'
    if (a.type && !(ADDRESS_TYPES as readonly string[]).includes(a.type)) e[`${p}.type`] = 'is not a valid type'
    const ls = lines(a.lines)
    if (ls.length > 5) e[`${p}.line`] = 'must have at most 5 items'
    else ls.forEach((l, j) => maxChars(e, `${p}.line[${j}]`, l, 200))
    maxChars(e, `${p}.text`, a.text.trim(), 500)
    maxChars(e, `${p}.city`, a.city.trim(), 100)
    maxChars(e, `${p}.district`, a.district.trim(), 100)
    maxChars(e, `${p}.state`, a.state.trim(), 100)
    maxChars(e, `${p}.postalCode`, a.postalCode.trim(), 20)
    maxChars(e, `${p}.country`, a.country.trim(), 100)
  })
  return e
}

/**
 * Turns the single search box into one of the API's exact-match patient
 * filters. The API stores names and contacts encrypted and supports no
 * free-text search: a date searches birth dates, anything with an @ or
 * mostly digits searches contacts, and the rest searches family names.
 */
export function parsePatientSearch(q: string): Pick<PatientQuery, 'family' | 'telecom' | 'birthdate'> {
  const s = q.trim()
  if (!s) return {}
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return { birthdate: s }
  if (s.includes('@')) return { telecom: s }
  const digits = s.replace(/\D/g, '')
  if (digits.length >= 4 && /^[\d\s()+.-]+$/.test(s)) return { telecom: s }
  return { family: s }
}
