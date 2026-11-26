<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { Gender, Page, Patient, PatientQuery } from '@/api/types'
import { GENDERS } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { age, formatDate, patientName, titleCase } from '@/lib/format'
import { parsePatientSearch } from '@/lib/patientForm'
import { compactQuery, qint, qs } from '@/lib/query'
import AppIcon from '@/components/AppIcon.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const filters = reactive({ q: '', gender: '', active: '' })
const page = ref<Page<Patient> | null>(null)
const loading = ref(false)
const error = ref<unknown>(null)

const pageNo = computed(() => qint(route.query, 'page', 1))
const limit = computed(() => qint(route.query, 'limit', 20, 1, 100))
const searchMode = computed(() => {
  const p = parsePatientSearch(filters.q)
  if (p.birthdate) return 'birth date'
  if (p.telecom) return 'phone or email'
  if (p.family) return 'family name'
  return ''
})

function syncFromRoute(): void {
  filters.q = qs(route.query, 'q')
  filters.gender = qs(route.query, 'gender')
  filters.active = qs(route.query, 'active')
}

async function load(): Promise<void> {
  loading.value = true
  error.value = null
  const query: PatientQuery = {
    page: pageNo.value,
    limit: limit.value,
    ...parsePatientSearch(qs(route.query, 'q')),
    gender: (GENDERS as readonly string[]).includes(qs(route.query, 'gender')) ? (qs(route.query, 'gender') as Gender) : undefined,
    active: qs(route.query, 'active') === 'true' ? true : qs(route.query, 'active') === 'false' ? false : undefined,
  }
  try {
    page.value = await api.patients.list(query)
  } catch (err) {
    error.value = err
    page.value = null
  } finally {
    loading.value = false
  }
}

function apply(extra: Record<string, string | number | undefined> = {}): void {
  void router.replace({
    query: compactQuery({ q: filters.q.trim(), gender: filters.gender, active: filters.active, limit: limit.value === 20 ? undefined : limit.value, ...extra }),
  })
}

function clear(): void {
  filters.q = ''
  filters.gender = ''
  filters.active = ''
  apply()
}

const hasFilters = computed(() => !!(qs(route.query, 'q') || qs(route.query, 'gender') || qs(route.query, 'active')))

watch(
  () => route.query,
  () => {
    syncFromRoute()
    void load()
  },
  { immediate: true },
)
</script>

<template>
  <div>
    <PageHeader title="Patients" description="Demographic records. Names and contact details are encrypted at rest.">
      <template #actions>
        <RouterLink v-if="auth.can('patients', 'create')" to="/patients/new" class="btn btn-primary">
          <AppIcon name="plus" class="size-4" /> New patient
        </RouterLink>
      </template>
    </PageHeader>

    <form class="card mb-4 grid gap-3 p-4 md:grid-cols-[1fr_auto_auto_auto]" role="search" @submit.prevent="apply()">
      <div>
        <label for="patient-search" class="sr-only">Search patients</label>
        <div class="relative">
          <AppIcon name="search" class="pointer-events-none absolute top-2.5 left-3 size-4 text-slate-400" />
          <input
            id="patient-search"
            v-model="filters.q"
            type="search"
            class="input pl-9"
            placeholder="Family name, phone, email or birth date (YYYY-MM-DD)"
            aria-describedby="patient-search-hint"
          />
        </div>
        <p id="patient-search-hint" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
          Exact match, not case sensitive.
          <span v-if="searchMode">Searching by {{ searchMode }}.</span>
        </p>
      </div>
      <div>
        <label for="patient-gender" class="sr-only">Gender</label>
        <select id="patient-gender" v-model="filters.gender" class="input" @change="apply()">
          <option value="">Any gender</option>
          <option v-for="g in GENDERS" :key="g" :value="g">{{ titleCase(g) }}</option>
        </select>
      </div>
      <div>
        <label for="patient-active" class="sr-only">Status</label>
        <select id="patient-active" v-model="filters.active" class="input" @change="apply()">
          <option value="">Active and inactive</option>
          <option value="true">Active only</option>
          <option value="false">Inactive only</option>
        </select>
      </div>
      <div class="flex items-start gap-2">
        <button type="submit" class="btn btn-primary">Search</button>
        <button v-if="hasFilters" type="button" class="btn btn-ghost" @click="clear">Clear</button>
      </div>
    </form>

    <ProblemAlert v-if="error" :error="error" class="mb-4" />

    <div class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="table-base" :aria-busy="loading">
          <caption class="sr-only">Patients</caption>
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col">Gender</th>
              <th scope="col">Birth date</th>
              <th scope="col" class="hidden md:table-cell">Contact</th>
              <th scope="col">Status</th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loading && !page">
              <tr v-for="i in 5" :key="i">
                <td colspan="5"><div class="h-4 animate-pulse rounded bg-slate-200 dark:bg-slate-800" /></td>
              </tr>
            </template>
            <tr v-for="p in page?.data ?? []" :key="p.id" class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
              <td>
                <RouterLink :to="`/patients/${p.id}`" class="link">{{ patientName(p) }}</RouterLink>
              </td>
              <td>{{ titleCase(p.gender) }}</td>
              <td class="whitespace-nowrap">
                <template v-if="p.birthDate">
                  {{ formatDate(p.birthDate) }}
                  <span class="text-slate-400">({{ age(p.birthDate) }}y)</span>
                </template>
                <span v-else class="text-slate-400">Not recorded</span>
              </td>
              <td class="hidden max-w-xs truncate md:table-cell">{{ p.telecom?.[0]?.value ?? '' }}</td>
              <td>
                <StatusBadge :tone="p.active ? 'success' : 'neutral'" dot>{{ p.active ? 'Active' : 'Inactive' }}</StatusBadge>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-if="page && page.data.length === 0" :title="hasFilters ? 'No matching patients' : 'No patients yet'" icon="patients">
        <template v-if="hasFilters">Filters match exactly. Check the spelling, or search by another field.</template>
        <template v-else-if="auth.can('patients', 'create')">Create the first patient record to get started.</template>
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
