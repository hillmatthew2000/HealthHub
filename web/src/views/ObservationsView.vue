<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { Observation, ObservationQuery, ObservationStatus, Page } from '@/api/types'
import { OBSERVATION_STATUSES } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { observationPatientId, titleCase } from '@/lib/format'
import { CATEGORIES } from '@/lib/observationForm'
import { usePatientNames } from '@/lib/patientNames'
import { compactQuery, qint, qs } from '@/lib/query'
import { isIsoDate, isUuid } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import EmptyState from '@/components/EmptyState.vue'
import ObservationTable from '@/components/ObservationTable.vue'
import PageHeader from '@/components/PageHeader.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const patientNames = usePatientNames()

const filters = reactive({ status: '', category: '', code: '', from: '', to: '', patient: '' })
const localErrors = reactive<Record<string, string>>({})
const page = ref<Page<Observation> | null>(null)
const loading = ref(false)
const error = ref<unknown>(null)

const keys = ['status', 'category', 'code', 'from', 'to', 'patient'] as const
const pageNo = computed(() => qint(route.query, 'page', 1))
const limit = computed(() => qint(route.query, 'limit', 20, 1, 100))
const hasFilters = computed(() => keys.some((k) => qs(route.query, k)))

async function load(): Promise<void> {
  loading.value = true
  error.value = null
  const status = qs(route.query, 'status')
  const query: ObservationQuery = {
    page: pageNo.value,
    limit: limit.value,
    status: (OBSERVATION_STATUSES as readonly string[]).includes(status) ? (status as ObservationStatus) : undefined,
    category: qs(route.query, 'category') || undefined,
    code: qs(route.query, 'code') || undefined,
    from: qs(route.query, 'from') || undefined,
    to: qs(route.query, 'to') || undefined,
    patient: qs(route.query, 'patient') || undefined,
  }
  try {
    page.value = await api.observations.list(query)
    patientNames.load(page.value.data.map(observationPatientId))
  } catch (err) {
    error.value = err
    page.value = null
  } finally {
    loading.value = false
  }
}

function validate(): boolean {
  for (const k of Object.keys(localErrors)) delete localErrors[k]
  if (filters.patient && !isUuid(filters.patient)) localErrors.patient = 'must be a patient ID (UUID)'
  if (filters.from && !isIsoDate(filters.from)) localErrors.from = 'must be a date'
  if (filters.to && !isIsoDate(filters.to)) localErrors.to = 'must be a date'
  if (filters.from && filters.to && filters.from > filters.to) localErrors.from = 'must not be after the end date'
  return Object.keys(localErrors).length === 0
}

function apply(extra: Record<string, string | number | undefined> = {}): void {
  if (!validate()) return
  void router.replace({
    query: compactQuery({
      ...Object.fromEntries(keys.map((k) => [k, filters[k].trim()])),
      limit: limit.value === 20 ? undefined : limit.value,
      ...extra,
    }),
  })
}

function clear(): void {
  for (const k of keys) filters[k] = ''
  apply()
}

watch(
  () => route.query,
  () => {
    for (const k of keys) filters[k] = qs(route.query, k)
    void load()
  },
  { immediate: true },
)
</script>

<template>
  <div>
    <PageHeader title="Observations" description="Lab results, vital signs and other clinical measurements, newest first.">
      <template #actions>
        <RouterLink v-if="auth.can('observations', 'create')" to="/observations/new" class="btn btn-primary">
          <AppIcon name="plus" class="size-4" /> New observation
        </RouterLink>
      </template>
    </PageHeader>

    <form class="card mb-4 p-4" role="search" aria-label="Filter observations" @submit.prevent="apply()">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
        <div>
          <label for="f-status" class="label">Status</label>
          <select id="f-status" v-model="filters.status" class="input">
            <option value="">Any</option>
            <option v-for="s in OBSERVATION_STATUSES" :key="s" :value="s">{{ titleCase(s) }}</option>
          </select>
        </div>
        <div>
          <label for="f-category" class="label">Category</label>
          <select id="f-category" v-model="filters.category" class="input">
            <option value="">Any</option>
            <option v-for="c in CATEGORIES" :key="c.code" :value="c.code">{{ c.display }}</option>
          </select>
        </div>
        <div>
          <label for="f-code" class="label">Code</label>
          <input id="f-code" v-model="filters.code" class="input" placeholder="8867-4 or system|code" />
        </div>
        <div>
          <label for="f-from" class="label">From</label>
          <input id="f-from" v-model="filters.from" type="date" class="input" :aria-invalid="localErrors.from ? 'true' : undefined" aria-describedby="f-from-err" />
          <p v-if="localErrors.from" id="f-from-err" class="mt-1 text-xs text-red-600">From {{ localErrors.from }}</p>
        </div>
        <div>
          <label for="f-to" class="label">To</label>
          <input id="f-to" v-model="filters.to" type="date" class="input" :aria-invalid="localErrors.to ? 'true' : undefined" />
        </div>
        <div>
          <label for="f-patient" class="label">Patient ID</label>
          <input id="f-patient" v-model="filters.patient" class="input font-mono" placeholder="UUID" :aria-invalid="localErrors.patient ? 'true' : undefined" aria-describedby="f-patient-err" />
          <p v-if="localErrors.patient" id="f-patient-err" class="mt-1 text-xs text-red-600">Patient ID {{ localErrors.patient }}</p>
        </div>
      </div>
      <div class="mt-3 flex justify-end gap-2">
        <button v-if="hasFilters" type="button" class="btn btn-ghost" @click="clear">Clear</button>
        <button type="submit" class="btn btn-primary"><AppIcon name="filter" class="size-4" /> Apply filters</button>
      </div>
    </form>

    <ProblemAlert v-if="error" :error="error" class="mb-4" />

    <div class="card overflow-hidden">
      <ObservationTable :observations="page?.data ?? []" :loading="loading" show-patient :patient-name="patientNames.nameOf" />
      <EmptyState v-if="page && page.data.length === 0" :title="hasFilters ? 'No matching observations' : 'No observations yet'" icon="beaker">
        <template v-if="hasFilters">Try widening the date range or removing a filter.</template>
      </EmptyState>
      <PaginationBar
        v-if="page && page.total > 0"
        :page="page.page"
        :limit="page.limit"
        :total="page.total"
        :total-pages="page.totalPages"
        @update:page="(p) => apply({ page: p > 1 ? p : undefined })"
        @update:limit="(l) => apply({ limit: l === 20 ? undefined : l })"
      />
    </div>
  </div>
</template>
