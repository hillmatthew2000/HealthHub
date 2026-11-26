<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, ApiError } from '@/api'
import type { Observation, Patient } from '@/api/types'
import { OBSERVATION_STATUSES } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { formatDateTime, observationTitle, patientName, titleCase } from '@/lib/format'
import {
  CATEGORIES,
  COMPARATORS,
  INTERPRETATIONS,
  emptyObservationForm,
  formToObservationInput,
  observationToForm,
  validateObservationForm,
  type ObservationForm,
} from '@/lib/observationForm'
import { qs } from '@/lib/query'
import { type Errors, fieldError, hasErrors, isUuid } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import FormField from '@/components/FormField.vue'
import ObservationStatus from '@/components/ObservationStatus.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'

const props = defineProps<{ id?: string }>()
const auth = useAuthStore()
const toast = useToastStore()
const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!props.id)
const readOnly = computed(() => (isEdit.value ? !auth.can('observations', 'update') : !auth.can('observations', 'create')))

const form = ref<ObservationForm>(emptyObservationForm())
const original = ref<Observation | null>(null)
const errors = ref<Errors>({})
const loadError = ref<unknown>(null)
const submitError = ref<unknown>(null)
const saving = ref(false)
const loaded = ref(false)

const patients = ref<Patient[]>([])
const patientTotal = ref(0)
const manualPatient = ref(false)

const confirmDelete = ref(false)
const deleting = ref(false)
const deleteError = ref<unknown>(null)

async function loadPatients(): Promise<void> {
  if (!auth.can('patients', 'read')) {
    manualPatient.value = true
    return
  }
  try {
    const page = await api.patients.list({ limit: 100, active: true })
    patients.value = page.data
    patientTotal.value = page.total
  } catch {
    manualPatient.value = true
  }
}

async function init(): Promise<void> {
  loaded.value = false
  loadError.value = null
  errors.value = {}
  if (props.id) {
    try {
      original.value = await api.observations.get(props.id)
      form.value = observationToForm(original.value)
    } catch (err) {
      loadError.value = err
      return
    }
  } else {
    original.value = null
    const pre = qs(route.query, 'patient')
    form.value = emptyObservationForm(isUuid(pre) ? pre : '')
  }
  if (!readOnly.value) await loadPatients()
  // Keep a preselected patient selectable even if it is not in the first page.
  const pid = form.value.patientId
  if (pid && !patients.value.some((p) => p.id === pid)) {
    if (auth.can('patients', 'read') && !readOnly.value) {
      try {
        patients.value = [await api.patients.get(pid), ...patients.value]
      } catch {
        manualPatient.value = true
      }
    }
  }
  loaded.value = true
}

watch(() => props.id, () => void init(), { immediate: true })

const err = (field: string) => fieldError(errors.value, field)

async function submit(): Promise<void> {
  submitError.value = null
  errors.value = validateObservationForm(form.value)
  if (hasErrors(errors.value)) {
    await nextTick()
    document.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
    return
  }
  saving.value = true
  try {
    const input = formToObservationInput(form.value, original.value ?? undefined)
    const saved = props.id ? await api.observations.update(props.id, input) : await api.observations.create(input)
    toast.success(props.id ? 'Observation updated.' : 'Observation recorded.')
    if (props.id) {
      original.value = saved
      form.value = observationToForm(saved)
    } else {
      await router.push(`/observations/${saved.id}`)
    }
  } catch (e) {
    submitError.value = e
    if (e instanceof ApiError && e.problem.errors?.length) errors.value = e.fieldErrors
  } finally {
    saving.value = false
  }
}

async function remove(): Promise<void> {
  if (!props.id) return
  deleting.value = true
  deleteError.value = null
  try {
    await api.observations.remove(props.id)
    toast.success('Observation deleted.')
    await router.push('/observations')
  } catch (e) {
    deleteError.value = e
  } finally {
    deleting.value = false
  }
}

const title = computed(() => {
  if (!isEdit.value) return 'New observation'
  if (!original.value) return 'Observation'
  return readOnly.value ? observationTitle(original.value) : `Edit ${observationTitle(original.value)}`
})

const backTo = computed(() =>
  !isEdit.value && form.value.patientId && qs(route.query, 'patient') ? `/patients/${form.value.patientId}` : '/observations',
)
</script>

<template>
  <div class="max-w-4xl">
    <RouterLink :to="backTo" class="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white">
      <AppIcon name="arrow-left" class="size-4" /> {{ backTo === '/observations' ? 'Observations' : 'Back to patient' }}
    </RouterLink>
    <PageHeader :title="title" :description="readOnly ? 'You can view this observation but not change it.' : 'Fields marked * are required.'">
      <template #actions>
        <ObservationStatus v-if="original" :status="original.status" />
        <RouterLink v-if="original && form.patientId" :to="`/patients/${form.patientId}`" class="btn btn-secondary">
          <AppIcon name="user" class="size-4" /> Patient
        </RouterLink>
        <button v-if="original && auth.can('observations', 'delete')" type="button" class="btn btn-danger" @click="confirmDelete = true">
          <AppIcon name="trash" class="size-4" /> Delete
        </button>
      </template>
    </PageHeader>

    <ProblemAlert v-if="loadError" :error="loadError" />

    <form v-else-if="loaded" class="space-y-6" novalidate @submit.prevent="submit">
      <ProblemAlert v-if="submitError" :error="submitError" title="Could not save the observation" />

      <fieldset :disabled="readOnly" class="space-y-6">
        <section class="card p-5" aria-labelledby="what-title">
          <h2 id="what-title" class="mb-4 font-semibold">What was observed</h2>
          <div class="grid gap-4 sm:grid-cols-6">
            <FormField v-slot="{ attrs }" label="Code system" class="sm:col-span-3" hint="For example http://loinc.org">
              <input v-bind="attrs" v-model="form.codeSystem" class="input" />
            </FormField>
            <FormField v-slot="{ attrs }" label="Code" :error="err('code')" class="sm:col-span-3" hint="Code or description is required.">
              <input v-bind="attrs" v-model="form.code" class="input font-mono" placeholder="8867-4" />
            </FormField>
            <FormField v-slot="{ attrs }" label="Display name" class="sm:col-span-3">
              <input v-bind="attrs" v-model="form.codeDisplay" class="input" placeholder="Heart rate" />
            </FormField>
            <FormField v-slot="{ attrs }" label="Description" :error="err('code.text')" class="sm:col-span-3">
              <input v-bind="attrs" v-model="form.codeText" class="input" maxlength="1000" />
            </FormField>
            <FormField v-slot="{ attrs }" label="Category" :error="err('category')" class="sm:col-span-2">
              <select v-bind="attrs" v-model="form.category" class="input">
                <option value="">None</option>
                <option v-for="c in CATEGORIES" :key="c.code" :value="c.code">{{ c.display }}</option>
              </select>
            </FormField>
            <FormField v-slot="{ attrs }" label="Status" :error="err('status')" class="sm:col-span-2" required>
              <select v-bind="attrs" v-model="form.status" class="input">
                <option v-for="s in OBSERVATION_STATUSES" :key="s" :value="s">{{ titleCase(s) }}</option>
              </select>
            </FormField>
            <FormField v-slot="{ attrs }" label="Effective" :error="err('effectiveDateTime')" class="sm:col-span-2" required>
              <input v-bind="attrs" v-model="form.effective" type="datetime-local" class="input" />
            </FormField>
          </div>
        </section>

        <section class="card p-5" aria-labelledby="subject-title">
          <h2 id="subject-title" class="mb-4 font-semibold">Patient</h2>
          <FormField v-if="!manualPatient && !readOnly" v-slot="{ attrs }" label="Patient" :error="err('subject.reference')" required>
            <select v-bind="attrs" v-model="form.patientId" class="input">
              <option value="" disabled>Select a patient</option>
              <option v-for="p in patients" :key="p.id" :value="p.id">
                {{ patientName(p) }}{{ p.birthDate ? ` (born ${p.birthDate})` : '' }}
              </option>
            </select>
          </FormField>
          <FormField v-else v-slot="{ attrs }" label="Patient ID" :error="err('subject.reference')" hint="The patient's UUID, shown on their record." required>
            <input v-bind="attrs" v-model="form.patientId" class="input font-mono" />
          </FormField>
          <p v-if="!readOnly" class="mt-2 text-xs text-slate-500 dark:text-slate-400">
            <template v-if="!manualPatient">
              Showing {{ patients.length }} of {{ patientTotal }} active patients, newest first.
            </template>
            <button type="button" class="link" @click="manualPatient = !manualPatient">
              {{ manualPatient ? 'Choose from the list instead' : 'Enter a patient ID instead' }}
            </button>
          </p>
        </section>

        <section class="card p-5" aria-labelledby="value-title">
          <h2 id="value-title" class="mb-4 font-semibold">Result</h2>
          <div class="grid gap-4 sm:grid-cols-6">
            <FormField v-slot="{ attrs }" label="Value type" class="sm:col-span-2">
              <select v-bind="attrs" v-model="form.valueType" class="input">
                <option value="quantity">Quantity</option>
                <option value="string">Text</option>
                <option value="integer">Whole number</option>
                <option value="boolean">Yes or no</option>
                <option value="none">No value</option>
                <option v-if="form.valueType === 'other'" value="other">Keep existing value</option>
              </select>
            </FormField>
            <template v-if="form.valueType === 'quantity'">
              <FormField v-slot="{ attrs }" label="Comparator" class="sm:col-span-1">
                <select v-bind="attrs" v-model="form.quantityComparator" class="input">
                  <option value="">=</option>
                  <option v-for="c in COMPARATORS" :key="c" :value="c">{{ c }}</option>
                </select>
              </FormField>
              <FormField v-slot="{ attrs }" label="Value" :error="err('valueQuantity')" class="sm:col-span-1" required>
                <input v-bind="attrs" v-model="form.quantityValue" class="input tabular-nums" inputmode="decimal" />
              </FormField>
              <FormField v-slot="{ attrs }" label="Unit" class="sm:col-span-2" hint="UCUM, for example mmol/L">
                <input v-bind="attrs" v-model="form.quantityUnit" class="input" />
              </FormField>
            </template>
            <FormField v-else-if="form.valueType === 'string'" v-slot="{ attrs }" label="Value" :error="err('valueString')" class="sm:col-span-4" required>
              <input v-bind="attrs" v-model="form.valueString" class="input" maxlength="10000" />
            </FormField>
            <FormField v-else-if="form.valueType === 'integer'" v-slot="{ attrs }" label="Value" :error="err('valueInteger')" class="sm:col-span-2" required>
              <input v-bind="attrs" v-model="form.valueInteger" class="input tabular-nums" inputmode="numeric" />
            </FormField>
            <FormField v-else-if="form.valueType === 'boolean'" v-slot="{ attrs }" label="Value" class="sm:col-span-2">
              <select v-bind="attrs" v-model="form.valueBoolean" class="input">
                <option value="true">Yes</option>
                <option value="false">No</option>
              </select>
            </FormField>
            <p v-else-if="form.valueType === 'other'" class="text-sm text-slate-500 sm:col-span-4 sm:self-end">
              This observation has a value type the editor does not support. It is kept unchanged.
            </p>
            <FormField v-slot="{ attrs }" label="Interpretation" class="sm:col-span-2">
              <select v-bind="attrs" v-model="form.interpretation" class="input">
                <option value="">None</option>
                <option v-for="i in INTERPRETATIONS" :key="i.code" :value="i.code">{{ i.display }} ({{ i.code }})</option>
              </select>
            </FormField>
            <FormField v-slot="{ attrs }" label="Note" :error="err('note')" class="sm:col-span-6">
              <textarea v-bind="attrs" v-model="form.note" rows="3" class="input" />
            </FormField>
          </div>
        </section>
      </fieldset>

      <p v-if="original" class="text-xs text-slate-500 dark:text-slate-400">
        Recorded {{ formatDateTime(original.createdAt) }}, last updated {{ formatDateTime(original.updatedAt) }}.
        <span class="font-mono select-all">{{ original.id }}</span>
      </p>

      <div v-if="!readOnly" class="flex justify-end gap-2">
        <RouterLink :to="backTo" class="btn btn-secondary">Cancel</RouterLink>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          {{ saving ? 'Saving...' : isEdit ? 'Save changes' : 'Record observation' }}
        </button>
      </div>
    </form>

    <div v-else class="card h-96 animate-pulse" aria-busy="true" />

    <ConfirmDialog
      :open="confirmDelete"
      title="Delete observation?"
      confirm-label="Delete observation"
      danger
      :busy="deleting"
      :error="deleteError"
      @cancel="(confirmDelete = false), (deleteError = null)"
      @confirm="remove"
    >
      This permanently deletes the observation. This cannot be undone.
    </ConfirmDialog>
  </div>
</template>
