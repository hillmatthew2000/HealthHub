import type { Observation, ObservationInput, ObservationStatus } from '@/api/types'
import { OBSERVATION_STATUSES } from '@/api/types'
import { observationPatientId } from './format'
import { type Errors, isUuid, maxChars } from './validation'

export const CATEGORY_SYSTEM = 'http://terminology.hl7.org/CodeSystem/observation-category'
export const INTERPRETATION_SYSTEM = 'http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation'
export const UCUM_SYSTEM = 'http://unitsofmeasure.org'

export const CATEGORIES = [
  { code: 'vital-signs', display: 'Vital Signs' },
  { code: 'laboratory', display: 'Laboratory' },
  { code: 'imaging', display: 'Imaging' },
  { code: 'social-history', display: 'Social History' },
  { code: 'survey', display: 'Survey' },
  { code: 'exam', display: 'Exam' },
  { code: 'procedure', display: 'Procedure' },
  { code: 'therapy', display: 'Therapy' },
  { code: 'activity', display: 'Activity' },
] as const

export const INTERPRETATIONS = [
  { code: 'N', display: 'Normal' },
  { code: 'A', display: 'Abnormal' },
  { code: 'AA', display: 'Critical abnormal' },
  { code: 'H', display: 'High' },
  { code: 'HH', display: 'Critical high' },
  { code: 'L', display: 'Low' },
  { code: 'LL', display: 'Critical low' },
] as const

export const COMPARATORS = ['<', '<=', '>=', '>', 'ad'] as const

/** Every value[x] element the API accepts. An observation has at most one. */
const VALUE_KEYS = [
  'valueQuantity',
  'valueCodeableConcept',
  'valueString',
  'valueBoolean',
  'valueInteger',
  'valueRange',
  'valueRatio',
  'valueTime',
  'valueDateTime',
  'valuePeriod',
] as const

function isManagedCategory(c: { coding?: { system?: string; code?: string }[] } | undefined): boolean {
  const coding = c?.coding?.[0]
  return coding?.system === CATEGORY_SYSTEM && CATEGORIES.some((x) => x.code === coding.code)
}

export type ValueType = 'none' | 'quantity' | 'string' | 'integer' | 'boolean' | 'other'

export interface ObservationForm {
  status: ObservationStatus | ''
  category: string
  codeSystem: string
  code: string
  codeDisplay: string
  codeText: string
  patientId: string
  /** A datetime-local value, in the browser's time zone. */
  effective: string
  valueType: ValueType
  quantityValue: string
  quantityUnit: string
  quantityComparator: string
  valueString: string
  valueInteger: string
  valueBoolean: 'true' | 'false'
  interpretation: string
  note: string
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/** Formats a timestamp for a datetime-local input in local time. */
export function toLocalInput(iso: string | undefined | Date): string {
  if (!iso) return ''
  const d = iso instanceof Date ? iso : new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function emptyObservationForm(patientId = '', now = new Date()): ObservationForm {
  return {
    status: 'final',
    category: 'vital-signs',
    codeSystem: 'http://loinc.org',
    code: '',
    codeDisplay: '',
    codeText: '',
    patientId,
    effective: toLocalInput(now),
    valueType: 'quantity',
    quantityValue: '',
    quantityUnit: '',
    quantityComparator: '',
    valueString: '',
    valueInteger: '',
    valueBoolean: 'true',
    interpretation: '',
    note: '',
  }
}

function detectValueType(o: Observation): ValueType {
  const present = VALUE_KEYS.filter((k) => o[k] !== undefined && o[k] !== null && o[k] !== '')
  if (present.length === 0) return 'none'
  if (present.length > 1) return 'other'
  switch (present[0]) {
    case 'valueQuantity':
      return 'quantity'
    case 'valueString':
      return 'string'
    case 'valueInteger':
      return 'integer'
    case 'valueBoolean':
      return 'boolean'
    default:
      return 'other'
  }
}

export function observationToForm(o: Observation): ObservationForm {
  const coding = o.code.coding?.[0]
  const firstCategory = o.category?.[0]
  const interp = o.interpretation?.[0]?.coding?.[0]
  return {
    status: o.status,
    category: isManagedCategory(firstCategory) ? (firstCategory?.coding[0]?.code ?? '') : '',
    codeSystem: coding?.system ?? '',
    code: coding?.code ?? '',
    codeDisplay: coding?.display ?? '',
    codeText: o.code.text ?? '',
    patientId: observationPatientId(o) ?? '',
    effective: toLocalInput(o.effectiveDateTime),
    valueType: detectValueType(o),
    quantityValue: o.valueQuantity?.value !== undefined ? String(o.valueQuantity.value) : '',
    quantityUnit: o.valueQuantity?.unit ?? '',
    quantityComparator: o.valueQuantity?.comparator ?? '',
    valueString: o.valueString ?? '',
    valueInteger: o.valueInteger !== undefined ? String(o.valueInteger) : '',
    valueBoolean: o.valueBoolean === false ? 'false' : 'true',
    interpretation: interp?.code ?? '',
    note: o.note?.[0]?.text ?? '',
  }
}

/**
 * Builds the request body. PUT replaces the whole resource, so an edit starts
 * from the stored observation and changes only what the form manages; other
 * FHIR elements (components, reference ranges and so on) are kept.
 */
export function formToObservationInput(f: ObservationForm, base?: Observation): ObservationInput {
  const out: Record<string, unknown> = { ...(base ?? {}) }
  for (const k of ['id', 'createdAt', 'updatedAt', 'createdBy']) delete out[k]

  out.status = f.status
  out.subject = { ...(base?.subject ?? {}), reference: `Patient/${f.patientId.trim()}` }
  out.effectiveDateTime = new Date(f.effective).toISOString()

  const coding = { ...(base?.code.coding?.[0] ?? {}) }
  coding.system = f.codeSystem.trim() || undefined
  coding.code = f.code.trim() || undefined
  coding.display = f.codeDisplay.trim() || undefined
  const codings = [coding, ...(base?.code.coding?.slice(1) ?? [])].filter((c) => c.code || c.display || c.system)
  out.code = { coding: codings.length ? codings : undefined, text: f.codeText.trim() || undefined }

  // The form manages the first category when it is a standard FHIR one;
  // any other categories are kept as they are.
  const existing = base?.category ?? []
  const kept = isManagedCategory(existing[0]) ? existing.slice(1) : existing
  const cat = CATEGORIES.find((c) => c.code === f.category)
  const categories = [
    ...(cat ? [{ coding: [{ system: CATEGORY_SYSTEM, code: cat.code, display: cat.display }] }] : []),
    ...kept,
  ]
  out.category = categories.length ? categories : undefined

  if (f.valueType !== 'other') {
    for (const k of VALUE_KEYS) delete out[k]
    switch (f.valueType) {
      case 'quantity': {
        const unit = f.quantityUnit.trim()
        out.valueQuantity = {
          value: Number(f.quantityValue),
          ...(f.quantityComparator ? { comparator: f.quantityComparator } : {}),
          ...(unit ? { unit, system: UCUM_SYSTEM, code: unit } : {}),
        }
        break
      }
      case 'string':
        out.valueString = f.valueString
        break
      case 'integer':
        out.valueInteger = Number(f.valueInteger)
        break
      case 'boolean':
        out.valueBoolean = f.valueBoolean === 'true'
        break
    }
  }

  const interp = INTERPRETATIONS.find((i) => i.code === f.interpretation)
  const restInterp = base?.interpretation?.slice(1) ?? []
  const interpretations = [
    ...(interp ? [{ coding: [{ system: INTERPRETATION_SYSTEM, code: interp.code, display: interp.display }] }] : []),
    ...restInterp,
  ]
  out.interpretation = interpretations.length ? interpretations : undefined

  const restNotes = base?.note?.slice(1) ?? []
  const notes = [...(f.note.trim() ? [{ ...(base?.note?.[0] ?? {}), text: f.note.trim() }] : []), ...restNotes]
  out.note = notes.length ? notes : undefined

  for (const k of Object.keys(out)) if (out[k] === undefined) delete out[k]
  return out as ObservationInput
}

/** Validates the form against internal/models/observation.go and the handler's extra rules. */
export function validateObservationForm(f: ObservationForm): Errors {
  const e: Errors = {}
  if (!f.status) e.status = 'is required'
  else if (!(OBSERVATION_STATUSES as readonly string[]).includes(f.status)) e.status = 'is not a valid status'
  if (!f.code.trim() && !f.codeText.trim()) e.code = 'must contain a coding with a code, or text'
  maxChars(e, 'code.text', f.codeText.trim(), 1000)
  if (!f.patientId.trim()) e['subject.reference'] = 'is required'
  else if (!isUuid(f.patientId)) e['subject.reference'] = 'must be Patient/<patient id>'
  if (!f.effective) e.effectiveDateTime = 'is required'
  else if (Number.isNaN(new Date(f.effective).getTime())) e.effectiveDateTime = 'is not a valid date and time'

  switch (f.valueType) {
    case 'quantity':
      if (f.quantityValue.trim() === '' || !Number.isFinite(Number(f.quantityValue))) e.valueQuantity = 'must be a number'
      if (f.quantityComparator && !(COMPARATORS as readonly string[]).includes(f.quantityComparator))
        e.valueQuantity = 'has an invalid comparator'
      break
    case 'string':
      if (!f.valueString.trim()) e.valueString = 'is required'
      maxChars(e, 'valueString', f.valueString, 10000)
      break
    case 'integer':
      if (!/^-?\d+$/.test(f.valueInteger.trim())) e.valueInteger = 'must be a whole number'
      break
  }
  return e
}
