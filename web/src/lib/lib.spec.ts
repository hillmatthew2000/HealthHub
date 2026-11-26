import { describe, expect, it } from 'vitest'
import type { Observation, Patient } from '@/api/types'
import { normalizeBaseUrl, readRuntimeConfig } from '@/config'
import { safeRedirect } from '@/router'
import { can } from './permissions'
import { emptyPatientForm, formToPatientInput, parsePatientSearch, patientToForm, validatePatientForm } from './patientForm'
import { emptyObservationForm, formToObservationInput, observationToForm, validateObservationForm } from './observationForm'
import { fieldError, isIsoDate, passwordProblem } from './validation'
import { isAbnormal, observationValue, patientName } from './format'

describe('permissions mirror internal/auth/roles.go', () => {
  it.each([
    ['admin', 'users', 'update', true],
    ['admin', 'audit-log', 'read', true],
    ['admin', 'audit-log', 'delete', false],
    ['practitioner', 'patients', 'update', true],
    ['practitioner', 'patients', 'delete', false],
    ['nurse', 'patients', 'read', true],
    ['nurse', 'observations', 'create', false],
    ['lab-tech', 'observations', 'update', true],
    ['lab-tech', 'patients', 'create', false],
    ['lab-tech', 'users', 'read', false],
  ] as const)('%s %s %s = %s', (role, resource, action, expected) => {
    expect(can([role], resource, action)).toBe(expected)
  })

  it('grants nothing to unknown or missing roles', () => {
    expect(can(['superuser'], 'patients', 'read')).toBe(false)
    expect(can(undefined, 'patients', 'read')).toBe(false)
  })

  it('combines roles', () => {
    expect(can(['nurse', 'lab-tech'], 'observations', 'create')).toBe(true)
  })
})

describe('runtime config', () => {
  it('defaults to same origin', () => {
    expect(readRuntimeConfig(undefined)).toEqual({ apiBaseUrl: '' })
    expect(readRuntimeConfig({})).toEqual({ apiBaseUrl: '' })
  })

  it('trims whitespace and trailing slashes', () => {
    expect(normalizeBaseUrl(' http://192.168.1.96:8081/ ')).toBe('http://192.168.1.96:8081')
    expect(normalizeBaseUrl(42)).toBe('')
  })
})

describe('safeRedirect', () => {
  it('allows in-app paths only', () => {
    expect(safeRedirect('/patients?page=2')).toBe('/patients?page=2')
    expect(safeRedirect('//evil.example')).toBe('/')
    expect(safeRedirect('/\\evil.example')).toBe('/')
    expect(safeRedirect('https://evil.example')).toBe('/')
    expect(safeRedirect(undefined)).toBe('/')
  })
})

describe('validation', () => {
  it('enforces 12 to 72 characters and at most 72 bytes', () => {
    expect(passwordProblem('short')).toMatch(/at least 12/)
    expect(passwordProblem('a'.repeat(12))).toBeUndefined()
    expect(passwordProblem('a'.repeat(72))).toBeUndefined()
    expect(passwordProblem('a'.repeat(73))).toMatch(/at most 72 characters/)
    expect(passwordProblem('é'.repeat(40))).toMatch(/bytes/)
  })

  it('checks real calendar dates', () => {
    expect(isIsoDate('2024-02-29')).toBe(true)
    expect(isIsoDate('2023-02-29')).toBe(false)
    expect(isIsoDate('2023-2-1')).toBe(false)
  })

  it('matches errors reported on child fields', () => {
    expect(fieldError({ 'name[0].given[1]': 'is required' }, 'name[0].given')).toBe('is required')
    expect(fieldError({ 'name[0].givenX': 'x' }, 'name[0].given')).toBeUndefined()
  })
})

describe('patient search', () => {
  it('maps the search box to exact-match filters', () => {
    expect(parsePatientSearch('  ')).toEqual({})
    expect(parsePatientSearch('1980-05-17')).toEqual({ birthdate: '1980-05-17' })
    expect(parsePatientSearch('jane@example.org')).toEqual({ telecom: 'jane@example.org' })
    expect(parsePatientSearch('+1 (555) 010-2000')).toEqual({ telecom: '+1 (555) 010-2000' })
    expect(parsePatientSearch('van der Berg')).toEqual({ family: 'van der Berg' })
  })
})

const patient: Patient = {
  id: 'p1',
  active: true,
  gender: 'female',
  birthDate: '1980-05-17',
  name: [
    { use: 'official', family: 'Doe', given: ['Jane', 'Q'], prefix: ['Dr'] },
    { use: 'maiden', family: 'Smith', given: ['Jane'] },
  ],
  telecom: [{ system: 'phone', value: '555-0100', use: 'mobile' }],
  address: [{ use: 'home', line: ['1 Main St', 'Apt 2'], city: 'Springfield', period: { start: '2020-01-01T00:00:00Z' } }],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('patient form', () => {
  it('round-trips a patient without losing fields it does not edit', () => {
    const input = formToPatientInput(patientToForm(patient))
    expect(input).toEqual({
      active: true,
      gender: 'female',
      birthDate: '1980-05-17',
      name: patient.name,
      telecom: patient.telecom,
      address: patient.address,
    })
    expect(patientName(patient)).toBe('Dr Jane Q Doe')
  })

  it('reports the same field paths as the API', () => {
    const f = emptyPatientForm()
    f.telecom.push({ system: 'email', value: '', use: '', rank: '-1' })
    f.address.push({ use: 'home', type: '', lines: '', city: 'x'.repeat(101), district: '', state: '', postalCode: '', country: '', text: '' })
    const e = validatePatientForm(f)
    expect(e).toMatchObject({
      gender: 'is required',
      'name[0].family': 'is required',
      'name[0].given': 'is required',
      'telecom[0].value': 'is required',
      'telecom[0].rank': expect.any(String),
      'address[0].city': 'must have at most 100 characters',
    })
  })

  it('accepts a minimal valid patient', () => {
    const f = emptyPatientForm()
    f.gender = 'unknown'
    f.family = 'Doe'
    f.given = 'John'
    expect(validatePatientForm(f)).toEqual({})
    expect(formToPatientInput(f)).toEqual({ active: true, gender: 'unknown', name: [{ use: 'official', family: 'Doe', given: ['John'] }] })
  })

  it('rejects birth dates in the future', () => {
    const f = { ...emptyPatientForm(), gender: 'male' as const, family: 'A', given: 'B', birthDate: '2999-01-01' }
    expect(validatePatientForm(f).birthDate).toBe('cannot be in the future')
  })
})

const PID = '0b6f2b36-1f0e-4a43-9d39-3f1f5cfa1a11'
const observation: Observation = {
  id: 'o1',
  status: 'final',
  category: [{ coding: [{ system: 'http://terminology.hl7.org/CodeSystem/observation-category', code: 'laboratory', display: 'Laboratory' }] }],
  code: { coding: [{ system: 'http://loinc.org', code: '2345-7', display: 'Glucose' }, { system: 'local', code: 'GLU' }] },
  subject: { reference: `Patient/${PID}` },
  effectiveDateTime: '2026-11-20T08:30:00.000Z',
  valueQuantity: { value: 7.2, unit: 'mmol/L', system: 'http://unitsofmeasure.org', code: 'mmol/L' },
  interpretation: [{ coding: [{ system: 'http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation', code: 'H', display: 'High' }] }],
  referenceRange: [{ low: { value: 3.9 }, high: { value: 5.6 } }],
  createdAt: '2026-11-20T08:31:00Z',
  updatedAt: '2026-11-20T08:31:00Z',
  createdBy: 'u1',
}

describe('observation form', () => {
  it('keeps elements the editor does not manage on update', () => {
    const input = formToObservationInput(observationToForm(observation), observation)
    expect(input).not.toHaveProperty('id')
    expect(input).not.toHaveProperty('createdBy')
    expect(input.referenceRange).toEqual(observation.referenceRange)
    expect(input.code.coding).toEqual(observation.code.coding)
    expect(input.category).toEqual(observation.category)
    expect(input.valueQuantity).toEqual(observation.valueQuantity)
    expect(input.interpretation).toEqual(observation.interpretation)
    expect(input.subject).toEqual({ reference: `Patient/${PID}` })
    expect(input.effectiveDateTime).toBe('2026-11-20T08:30:00.000Z')
    expect(isAbnormal(observation)).toBe(true)
    expect(observationValue(observation)).toBe('7.2 mmol/L')
  })

  it('replaces the value when the value type changes', () => {
    const f = observationToForm(observation)
    f.valueType = 'string'
    f.valueString = 'Positive'
    const input = formToObservationInput(f, observation)
    expect(input.valueString).toBe('Positive')
    expect(input).not.toHaveProperty('valueQuantity')
  })

  it('applies the handler rules for code and subject', () => {
    const f = emptyObservationForm()
    f.patientId = 'not-a-uuid'
    expect(validateObservationForm(f)).toMatchObject({
      code: 'must contain a coding with a code, or text',
      'subject.reference': 'must be Patient/<patient id>',
      valueQuantity: 'must be a number',
    })
    f.patientId = PID
    f.codeText = 'Heart rate'
    f.quantityValue = '72'
    expect(validateObservationForm(f)).toEqual({})
    const input = formToObservationInput(f)
    expect(input).toMatchObject({ status: 'final', subject: { reference: `Patient/${PID}` }, code: { text: 'Heart rate' }, valueQuantity: { value: 72 } })
  })
})
