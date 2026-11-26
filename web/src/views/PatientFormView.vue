<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, ApiError } from '@/api'
import type { Patient } from '@/api/types'
import { ADDRESS_TYPES, ADDRESS_USES, GENDERS, NAME_USES, TELECOM_SYSTEMS, TELECOM_USES } from '@/api/types'
import { useToastStore } from '@/stores/toast'
import { formatName, patientName, titleCase } from '@/lib/format'
import {
  emptyAddress,
  emptyPatientForm,
  emptyTelecom,
  formToPatientInput,
  patientToForm,
  validatePatientForm,
  type PatientForm,
} from '@/lib/patientForm'
import { type Errors, fieldError, hasErrors } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const toast = useToastStore()

const isEdit = computed(() => !!props.id)
const form = ref<PatientForm>(emptyPatientForm())
const original = ref<Patient | null>(null)
const errors = ref<Errors>({})
const loadError = ref<unknown>(null)
const submitError = ref<unknown>(null)
const saving = ref(false)
const loaded = ref(!props.id)
const today = new Date().toISOString().slice(0, 10)

watch(
  () => props.id,
  async (id) => {
    if (!id) {
      form.value = emptyPatientForm()
      loaded.value = true
      return
    }
    loaded.value = false
    try {
      original.value = await api.patients.get(id)
      form.value = patientToForm(original.value)
      loaded.value = true
    } catch (err) {
      loadError.value = err
    }
  },
  { immediate: true },
)

const err = (field: string) => fieldError(errors.value, field)

async function focusFirstError(): Promise<void> {
  await nextTick()
  document.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
}

async function submit(): Promise<void> {
  submitError.value = null
  errors.value = validatePatientForm(form.value)
  if (hasErrors(errors.value)) {
    void focusFirstError()
    return
  }
  saving.value = true
  try {
    const input = formToPatientInput(form.value)
    const saved = props.id ? await api.patients.update(props.id, input) : await api.patients.create(input)
    toast.success(props.id ? 'Patient updated.' : 'Patient created.')
    await router.push(`/patients/${saved.id}`)
  } catch (e) {
    submitError.value = e
    if (e instanceof ApiError && e.problem.errors?.length) {
      errors.value = e.fieldErrors
      void focusFirstError()
    }
  } finally {
    saving.value = false
  }
}

const cancelTo = computed(() => (props.id ? `/patients/${props.id}` : '/patients'))
</script>

<template>
  <div class="max-w-4xl">
    <RouterLink :to="cancelTo" class="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white">
      <AppIcon name="arrow-left" class="size-4" /> {{ isEdit ? 'Back to patient' : 'Patients' }}
    </RouterLink>
    <PageHeader
      :title="isEdit ? (original ? `Edit ${patientName(original)}` : 'Edit patient') : 'New patient'"
      description="Fields marked * are required."
    />

    <ProblemAlert v-if="loadError" :error="loadError" />

    <form v-else-if="loaded" class="space-y-6" novalidate @submit.prevent="submit">
      <ProblemAlert v-if="submitError" :error="submitError" :title="isEdit ? 'Could not save the patient' : 'Could not create the patient'" />

      <section class="card p-5" aria-labelledby="name-title">
        <h2 id="name-title" class="mb-4 font-semibold">Name</h2>
        <div class="grid gap-4 sm:grid-cols-2">
          <FormField v-slot="{ attrs }" label="Given names" :error="err('name[0].given')" hint="Separate multiple given names with spaces." required>
            <input v-bind="attrs" v-model="form.given" class="input" autocomplete="off" />
          </FormField>
          <FormField v-slot="{ attrs }" label="Family name" :error="err('name[0].family')" required>
            <input v-bind="attrs" v-model="form.family" class="input" maxlength="200" autocomplete="off" />
          </FormField>
          <FormField v-slot="{ attrs }" label="Prefix" :error="err('name[0].prefix')" hint="For example Dr or Ms.">
            <input v-bind="attrs" v-model="form.prefix" class="input" autocomplete="off" />
          </FormField>
          <FormField v-slot="{ attrs }" label="Suffix" :error="err('name[0].suffix')">
            <input v-bind="attrs" v-model="form.suffix" class="input" autocomplete="off" />
          </FormField>
          <FormField v-slot="{ attrs }" label="Name use" :error="err('name[0].use')">
            <select v-bind="attrs" v-model="form.nameUse" class="input">
              <option value="">Not specified</option>
              <option v-for="u in NAME_USES" :key="u" :value="u">{{ titleCase(u) }}</option>
            </select>
          </FormField>
        </div>
        <p v-if="form.otherNames.length" class="mt-4 text-sm text-slate-500 dark:text-slate-400">
          Other recorded names are kept unchanged: {{ form.otherNames.map(formatName).join('; ') }}.
        </p>
      </section>

      <section class="card p-5" aria-labelledby="details-title">
        <h2 id="details-title" class="mb-4 font-semibold">Details</h2>
        <div class="grid gap-4 sm:grid-cols-3">
          <FormField v-slot="{ attrs }" label="Gender" :error="err('gender')" required>
            <select v-bind="attrs" v-model="form.gender" class="input">
              <option value="" disabled>Select</option>
              <option v-for="g in GENDERS" :key="g" :value="g">{{ titleCase(g) }}</option>
            </select>
          </FormField>
          <FormField v-slot="{ attrs }" label="Birth date" :error="err('birthDate')">
            <input v-bind="attrs" v-model="form.birthDate" type="date" class="input" :max="today" />
          </FormField>
          <div class="flex items-end pb-2">
            <label class="inline-flex items-center gap-2 text-sm">
              <input v-model="form.active" type="checkbox" class="size-4 rounded border-slate-300 accent-brand-600" />
              Active record
            </label>
          </div>
        </div>
      </section>

      <section class="card p-5" aria-labelledby="telecom-title">
        <div class="mb-4 flex items-center justify-between">
          <h2 id="telecom-title" class="font-semibold">Contact details</h2>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="form.telecom.length >= 20" @click="form.telecom.push(emptyTelecom())">
            <AppIcon name="plus" class="size-4" /> Add contact
          </button>
        </div>
        <p v-if="err('telecom')" class="mb-2 text-xs text-red-600">Contacts {{ err('telecom') }}</p>
        <p v-if="form.telecom.length === 0" class="text-sm text-slate-500">No contact details.</p>
        <fieldset v-for="(t, i) in form.telecom" :key="i" class="grid items-start gap-3 border-t border-slate-100 py-3 first-of-type:border-0 sm:grid-cols-[8rem_1fr_8rem_5rem_auto] dark:border-slate-800">
          <legend class="sr-only">Contact {{ i + 1 }}</legend>
          <FormField v-slot="{ attrs }" label="Type" :error="err(`telecom[${i}].system`)" required>
            <select v-bind="attrs" v-model="t.system" class="input">
              <option v-for="s in TELECOM_SYSTEMS" :key="s" :value="s">{{ titleCase(s) }}</option>
            </select>
          </FormField>
          <FormField v-slot="{ attrs }" label="Value" :error="err(`telecom[${i}].value`)" required>
            <input v-bind="attrs" v-model="t.value" class="input" maxlength="254" :type="t.system === 'email' ? 'email' : t.system === 'url' ? 'url' : 'text'" />
          </FormField>
          <FormField v-slot="{ attrs }" label="Use" :error="err(`telecom[${i}].use`)">
            <select v-bind="attrs" v-model="t.use" class="input">
              <option value="">Any</option>
              <option v-for="u in TELECOM_USES" :key="u" :value="u">{{ titleCase(u) }}</option>
            </select>
          </FormField>
          <FormField v-slot="{ attrs }" label="Rank" :error="err(`telecom[${i}].rank`)">
            <input v-bind="attrs" v-model="t.rank" class="input" inputmode="numeric" />
          </FormField>
          <button type="button" class="btn btn-ghost mt-6 px-2" :aria-label="`Remove contact ${i + 1}`" @click="form.telecom.splice(i, 1)">
            <AppIcon name="trash" class="size-4" />
          </button>
        </fieldset>
      </section>

      <section class="card p-5" aria-labelledby="address-title">
        <div class="mb-4 flex items-center justify-between">
          <h2 id="address-title" class="font-semibold">Addresses</h2>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="form.address.length >= 10" @click="form.address.push(emptyAddress())">
            <AppIcon name="plus" class="size-4" /> Add address
          </button>
        </div>
        <p v-if="form.address.length === 0" class="text-sm text-slate-500">No addresses.</p>
        <fieldset v-for="(a, i) in form.address" :key="i" class="border-t border-slate-100 py-4 first-of-type:border-0 dark:border-slate-800">
          <legend class="sr-only">Address {{ i + 1 }}</legend>
          <div class="grid gap-3 sm:grid-cols-6">
            <FormField v-slot="{ attrs }" label="Use" :error="err(`address[${i}].use`)" class="sm:col-span-2">
              <select v-bind="attrs" v-model="a.use" class="input">
                <option value="">Any</option>
                <option v-for="u in ADDRESS_USES" :key="u" :value="u">{{ titleCase(u) }}</option>
              </select>
            </FormField>
            <FormField v-slot="{ attrs }" label="Type" :error="err(`address[${i}].type`)" class="sm:col-span-2">
              <select v-bind="attrs" v-model="a.type" class="input">
                <option value="">Any</option>
                <option v-for="t in ADDRESS_TYPES" :key="t" :value="t">{{ titleCase(t) }}</option>
              </select>
            </FormField>
            <div class="flex items-end justify-end sm:col-span-2">
              <button type="button" class="btn btn-ghost btn-sm" @click="form.address.splice(i, 1)">
                <AppIcon name="trash" class="size-4" /> Remove address
              </button>
            </div>
            <FormField v-slot="{ attrs }" label="Street lines" :error="err(`address[${i}].line`)" hint="Up to 5 lines." class="sm:col-span-6">
              <textarea v-bind="attrs" v-model="a.lines" rows="2" class="input" />
            </FormField>
            <FormField v-slot="{ attrs }" label="City" :error="err(`address[${i}].city`)" class="sm:col-span-2">
              <input v-bind="attrs" v-model="a.city" class="input" maxlength="100" />
            </FormField>
            <FormField v-slot="{ attrs }" label="District" :error="err(`address[${i}].district`)" class="sm:col-span-2">
              <input v-bind="attrs" v-model="a.district" class="input" maxlength="100" />
            </FormField>
            <FormField v-slot="{ attrs }" label="State" :error="err(`address[${i}].state`)" class="sm:col-span-2">
              <input v-bind="attrs" v-model="a.state" class="input" maxlength="100" />
            </FormField>
            <FormField v-slot="{ attrs }" label="Postal code" :error="err(`address[${i}].postalCode`)" class="sm:col-span-2">
              <input v-bind="attrs" v-model="a.postalCode" class="input" maxlength="20" />
            </FormField>
            <FormField v-slot="{ attrs }" label="Country" :error="err(`address[${i}].country`)" class="sm:col-span-4">
              <input v-bind="attrs" v-model="a.country" class="input" maxlength="100" />
            </FormField>
          </div>
        </fieldset>
      </section>

      <div class="flex justify-end gap-2">
        <RouterLink :to="cancelTo" class="btn btn-secondary">Cancel</RouterLink>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          {{ saving ? 'Saving...' : isEdit ? 'Save changes' : 'Create patient' }}
        </button>
      </div>
    </form>

    <div v-else class="card h-96 animate-pulse" aria-busy="true" />
  </div>
</template>
