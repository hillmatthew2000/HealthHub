import { reactive } from 'vue'
import { api } from '@/api'
import { patientName } from './format'

// Observations reference their patient by ID only. This resolves IDs to
// display names for the current page, one request per patient, cached for
// the page session. Every lookup is a patient read the API audits.

const names = reactive(new Map<string, string>())
const pending = new Map<string, Promise<void>>()

export function usePatientNames() {
  function load(ids: Iterable<string | undefined>): void {
    for (const id of new Set(ids)) {
      if (!id || names.has(id) || pending.has(id)) continue
      const p = api.patients
        .get(id)
        .then((pt) => {
          names.set(id, patientName(pt))
        })
        .catch(() => {
          // Unknown or inaccessible patient: fall back to the short ID.
        })
        .finally(() => pending.delete(id))
      pending.set(id, p)
    }
  }

  function nameOf(id: string | undefined): string | undefined {
    return id ? names.get(id) : undefined
  }

  return { load, nameOf }
}

export function clearPatientNames(): void {
  names.clear()
  pending.clear()
}
