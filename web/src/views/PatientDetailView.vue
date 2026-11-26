<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api'
import type { Observation, Page, Patient } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { age, formatDate, formatDateTime, formatName, patientName, titleCase } from '@/lib/format'
import AppIcon from '@/components/AppIcon.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import EmptyState from '@/components/EmptyState.vue'
import ObservationTable from '@/components/ObservationTable.vue'
import PageHeader from '@/components/PageHeader.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const props = defineProps<{ id: string }>()
const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()

const patient = ref<Patient | null>(null)
const error = ref<unknown>(null)
const tab = ref<'overview' | 'observations'>('overview')

const obs = ref<Page<Observation> | null>(null)
const obsError = ref<unknown>(null)
const obsLoading = ref(false)
const obsPage = ref(1)
const obsLimit = ref(20)

const confirmDelete = ref(false)
const deleting = ref(false)
const deleteError = ref<unknown>(null)

const canObservations = computed(() => auth.can('observations', 'read'))

async function load(): Promise<void> {
  error.value = null
  patient.value = null
  try {
    patient.value = await api.patients.get(props.id)
  } catch (err) {
    error.value = err
  }
}

async function loadObservations(): Promise<void> {
  if (!canObservations.value) return
  obsLoading.value = true
  obsError.value = null
  try {
    obs.value = await api.patients.observations(props.id, { page: obsPage.value, limit: obsLimit.value })
  } catch (err) {
    obsError.value = err
  } finally {
    obsLoading.value = false
  }
}

watch(() => props.id, () => {
  tab.value = 'overview'
  obs.value = null
  obsPage.value = 1
  void load()
}, { immediate: true })

watch([tab, obsPage, obsLimit], () => {
  if (tab.value === 'observations') void loadObservations()
})

async function remove(): Promise<void> {
  deleting.value = true
  deleteError.value = null
  try {
    await api.patients.remove(props.id)
    toast.success('Patient deleted.')
    await router.push('/patients')
  } catch (err) {
    deleteError.value = err
  } finally {
    deleting.value = false
  }
}

function onTabKey(e: KeyboardEvent): void {
  if (!canObservations.value) return
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
    tab.value = tab.value === 'overview' ? 'observations' : 'overview'
    document.getElementById(`tab-${tab.value}`)?.focus()
  }
}
</script>

<template>
  <div>
    <RouterLink to="/patients" class="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white">
      <AppIcon name="arrow-left" class="size-4" /> Patients
    </RouterLink>

    <ProblemAlert v-if="error" :error="error" />

    <template v-else-if="patient">
      <PageHeader :title="patientName(patient)">
        <template #eyebrow>
          <div class="mb-1 flex flex-wrap items-center gap-2">
            <StatusBadge :tone="patient.active ? 'success' : 'neutral'" dot>{{ patient.active ? 'Active' : 'Inactive' }}</StatusBadge>
            <span class="text-sm text-slate-500 dark:text-slate-400">
              {{ titleCase(patient.gender) }}<template v-if="patient.birthDate"> · {{ age(patient.birthDate) }} years</template>
            </span>
          </div>
        </template>
        <template #actions>
          <RouterLink
            v-if="auth.can('observations', 'create')"
            :to="{ path: '/observations/new', query: { patient: patient.id } }"
            class="btn btn-secondary"
          >
            <AppIcon name="plus" class="size-4" /> Observation
          </RouterLink>
          <RouterLink v-if="auth.can('patients', 'update')" :to="`/patients/${patient.id}/edit`" class="btn btn-secondary">
            <AppIcon name="pencil" class="size-4" /> Edit
          </RouterLink>
          <button v-if="auth.can('patients', 'delete')" type="button" class="btn btn-danger" @click="confirmDelete = true">
            <AppIcon name="trash" class="size-4" /> Delete
          </button>
        </template>
      </PageHeader>

      <div role="tablist" aria-label="Patient record" class="mb-6 flex gap-1 border-b border-slate-200 dark:border-slate-800" @keydown="onTabKey">
        <button
          id="tab-overview"
          type="button"
          role="tab"
          :aria-selected="tab === 'overview'"
          :tabindex="tab === 'overview' ? 0 : -1"
          aria-controls="panel-overview"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium',
            tab === 'overview' ? 'border-brand-600 text-brand-700 dark:text-brand-300' : 'border-transparent text-slate-500 hover:text-slate-800 dark:hover:text-white',
          ]"
          @click="tab = 'overview'"
        >
          Overview
        </button>
        <button
          v-if="canObservations"
          id="tab-observations"
          type="button"
          role="tab"
          :aria-selected="tab === 'observations'"
          :tabindex="tab === 'observations' ? 0 : -1"
          aria-controls="panel-observations"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium',
            tab === 'observations' ? 'border-brand-600 text-brand-700 dark:text-brand-300' : 'border-transparent text-slate-500 hover:text-slate-800 dark:hover:text-white',
          ]"
          @click="tab = 'observations'"
        >
          Observations<span v-if="obs" class="ml-1.5 rounded-full bg-slate-100 px-1.5 text-xs dark:bg-slate-800">{{ obs.total }}</span>
        </button>
      </div>

      <div v-show="tab === 'overview'" id="panel-overview" role="tabpanel" aria-labelledby="tab-overview" class="grid gap-6 lg:grid-cols-3">
        <section class="card p-5 lg:col-span-2" aria-labelledby="demo-title">
          <h2 id="demo-title" class="mb-4 font-semibold">Demographics</h2>
          <dl class="grid gap-x-6 gap-y-4 sm:grid-cols-2">
            <div v-for="(n, i) in patient.name" :key="i">
              <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">{{ n.use ? `${titleCase(n.use)} name` : 'Name' }}</dt>
              <dd class="mt-0.5">{{ formatName(n) }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Gender</dt>
              <dd class="mt-0.5">{{ titleCase(patient.gender) }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Birth date</dt>
              <dd class="mt-0.5">{{ patient.birthDate ? formatDate(patient.birthDate) : 'Not recorded' }}</dd>
            </div>
          </dl>

          <h3 class="mt-6 mb-3 text-sm font-semibold">Contact</h3>
          <ul v-if="patient.telecom?.length" class="space-y-2 text-sm">
            <li v-for="(t, i) in patient.telecom" :key="i" class="flex flex-wrap items-center gap-2">
              <StatusBadge>{{ titleCase(t.system) }}</StatusBadge>
              <span>{{ t.value }}</span>
              <span v-if="t.use" class="text-slate-400">({{ t.use }})</span>
            </li>
          </ul>
          <p v-else class="text-sm text-slate-500">No contact details recorded.</p>

          <h3 class="mt-6 mb-3 text-sm font-semibold">Address</h3>
          <div v-if="patient.address?.length" class="grid gap-4 sm:grid-cols-2">
            <address v-for="(a, i) in patient.address" :key="i" class="text-sm not-italic">
              <p v-if="a.use || a.type" class="mb-1 text-xs text-slate-500 uppercase">{{ [a.use, a.type].filter(Boolean).join(' · ') }}</p>
              <p v-if="a.text">{{ a.text }}</p>
              <p v-for="(l, j) in a.line ?? []" :key="j">{{ l }}</p>
              <p>{{ [a.city, a.district, a.state, a.postalCode].filter(Boolean).join(', ') }}</p>
              <p v-if="a.country">{{ a.country }}</p>
            </address>
          </div>
          <p v-else class="text-sm text-slate-500">No address recorded.</p>
        </section>

        <section class="card p-5" aria-labelledby="record-title">
          <h2 id="record-title" class="mb-4 font-semibold">Record</h2>
          <dl class="space-y-3 text-sm">
            <div>
              <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Patient ID</dt>
              <dd class="mt-0.5 font-mono text-xs break-all select-all">{{ patient.id }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Created</dt>
              <dd class="mt-0.5">{{ formatDateTime(patient.createdAt) }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Last updated</dt>
              <dd class="mt-0.5">{{ formatDateTime(patient.updatedAt) }}</dd>
            </div>
          </dl>
        </section>
      </div>

      <div v-if="canObservations" v-show="tab === 'observations'" id="panel-observations" role="tabpanel" aria-labelledby="tab-observations">
        <ProblemAlert v-if="obsError" :error="obsError" class="mb-4" />
        <div class="card overflow-hidden">
          <ObservationTable :observations="obs?.data ?? []" :loading="obsLoading" />
          <EmptyState v-if="obs && obs.data.length === 0" title="No observations" icon="beaker">
            Nothing has been recorded for this patient yet.
          </EmptyState>
          <PaginationBar
            v-if="obs && obs.total > 0"
            :page="obs.page"
            :limit="obs.limit"
            :total="obs.total"
            :total-pages="obs.totalPages"
            @update:page="(p) => (obsPage = p)"
            @update:limit="(l) => ((obsLimit = l), (obsPage = 1))"
          />
        </div>
      </div>

      <ConfirmDialog
        :open="confirmDelete"
        title="Delete patient?"
        confirm-label="Delete patient"
        danger
        :busy="deleting"
        :error="deleteError"
        @cancel="(confirmDelete = false), (deleteError = null)"
        @confirm="remove"
      >
        This permanently deletes <strong>{{ patientName(patient) }}</strong> and all of their observations. This cannot be undone.
      </ConfirmDialog>
    </template>

    <div v-else class="space-y-4" aria-busy="true">
      <div class="h-8 w-64 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
      <div class="card h-64 animate-pulse" />
    </div>
  </div>
</template>
