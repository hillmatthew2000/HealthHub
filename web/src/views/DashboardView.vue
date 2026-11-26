<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '@/api'
import type { Observation } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime, isAbnormal, observationPatientId, observationTitle, observationValue, shortId, userName } from '@/lib/format'
import { usePatientNames } from '@/lib/patientNames'
import AppIcon, { type IconName } from '@/components/AppIcon.vue'
import EmptyState from '@/components/EmptyState.vue'
import HealthBadge from '@/components/HealthBadge.vue'
import ObservationStatus from '@/components/ObservationStatus.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const auth = useAuthStore()
const patientNames = usePatientNames()

interface Stat {
  key: string
  label: string
  icon: IconName
  to: string
  value: number | null
  error?: unknown
}

const stats = ref<Stat[]>([])
const recent = ref<Observation[]>([])
const recentError = ref<unknown>(null)
const loadingRecent = ref(true)

const greeting = computed(() => {
  const h = new Date().getHours()
  const part = h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'
  return auth.user ? `${part}, ${auth.user.firstName || userName(auth.user)}` : part
})

const canPatients = computed(() => auth.can('patients', 'read'))
const canObservations = computed(() => auth.can('observations', 'read'))
const noAccess = computed(() => !canPatients.value && !canObservations.value && !auth.isAdmin)

async function loadStats(): Promise<void> {
  const defs: (Omit<Stat, 'value'> & { load: () => Promise<number> })[] = []
  if (canPatients.value)
    defs.push({ key: 'patients', label: 'Patients', icon: 'patients', to: '/patients', load: async () => (await api.patients.list({ limit: 1 })).total })
  if (canPatients.value)
    defs.push({
      key: 'active',
      label: 'Active patients',
      icon: 'heart',
      to: '/patients?active=true',
      load: async () => (await api.patients.list({ limit: 1, active: true })).total,
    })
  if (canObservations.value)
    defs.push({
      key: 'observations',
      label: 'Observations',
      icon: 'beaker',
      to: '/observations',
      load: async () => (await api.observations.list({ limit: 1 })).total,
    })
  if (auth.can('users', 'read'))
    defs.push({ key: 'users', label: 'Staff accounts', icon: 'users', to: '/users', load: async () => (await api.users.list({ limit: 1 })).total })

  stats.value = defs.map(({ load: _load, ...d }) => ({ ...d, value: null }))
  await Promise.all(
    defs.map(async (d, i) => {
      const stat = stats.value[i]
      if (!stat) return
      try {
        stat.value = await d.load()
      } catch (err) {
        stat.error = err
      }
    }),
  )
}

async function loadRecent(): Promise<void> {
  if (!canObservations.value) return
  loadingRecent.value = true
  try {
    recent.value = (await api.observations.list({ limit: 6 })).data
    patientNames.load(recent.value.map(observationPatientId))
  } catch (err) {
    recentError.value = err
  } finally {
    loadingRecent.value = false
  }
}

onMounted(() => {
  void loadStats()
  void loadRecent()
})
</script>

<template>
  <div>
    <PageHeader :title="greeting" description="An overview of HealthHub records you can access.">
      <template #actions>
        <HealthBadge detailed />
        <RouterLink v-if="auth.can('patients', 'create')" to="/patients/new" class="btn btn-secondary">
          <AppIcon name="plus" class="size-4" /> New patient
        </RouterLink>
        <RouterLink v-if="auth.can('observations', 'create')" to="/observations/new" class="btn btn-primary">
          <AppIcon name="plus" class="size-4" /> New observation
        </RouterLink>
      </template>
    </PageHeader>

    <div v-if="noAccess" class="card">
      <EmptyState title="No access yet" icon="shield">
        Your account has no roles, so there is nothing to show. Ask an administrator to assign you a role.
      </EmptyState>
    </div>

    <template v-else>
      <section aria-label="Totals" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <RouterLink
          v-for="s in stats"
          :key="s.key"
          :to="s.to"
          class="card group flex items-center gap-4 p-5 transition hover:border-brand-300 hover:shadow-md dark:hover:border-brand-700"
        >
          <div class="rounded-lg bg-brand-50 p-2.5 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300">
            <AppIcon :name="s.icon" class="size-6" />
          </div>
          <div>
            <p class="text-sm text-slate-500 dark:text-slate-400">{{ s.label }}</p>
            <p class="text-2xl font-semibold text-slate-900 tabular-nums dark:text-white">
              <span v-if="s.error" class="text-base text-red-600 dark:text-red-400" :title="String(s.error)">Unavailable</span>
              <span v-else-if="s.value === null" class="inline-block h-7 w-12 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
              <span v-else>{{ s.value.toLocaleString() }}</span>
            </p>
          </div>
        </RouterLink>
      </section>

      <section v-if="canObservations" class="card mt-8 overflow-hidden" aria-labelledby="recent-title">
        <div class="flex items-center justify-between border-b border-slate-200 px-5 py-4 dark:border-slate-800">
          <h2 id="recent-title" class="font-semibold text-slate-900 dark:text-white">Recent observations</h2>
          <RouterLink to="/observations" class="link text-sm">View all</RouterLink>
        </div>
        <div v-if="recentError" class="p-5"><ProblemAlert :error="recentError" /></div>
        <ul v-else-if="loadingRecent" class="divide-y divide-slate-100 dark:divide-slate-800" aria-busy="true">
          <li v-for="i in 4" :key="i" class="flex items-center gap-4 px-5 py-4">
            <div class="h-4 w-1/3 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
            <div class="ml-auto h-4 w-20 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
          </li>
        </ul>
        <EmptyState v-else-if="recent.length === 0" title="No observations yet" icon="beaker">
          Observations recorded for patients appear here.
        </EmptyState>
        <ul v-else class="divide-y divide-slate-100 dark:divide-slate-800">
          <li v-for="o in recent" :key="o.id">
            <RouterLink
              :to="`/observations/${o.id}`"
              class="flex flex-col gap-2 px-5 py-3.5 hover:bg-slate-50 sm:flex-row sm:items-center sm:gap-4 dark:hover:bg-slate-800/50"
            >
              <div class="min-w-0 flex-1">
                <p class="truncate font-medium text-slate-900 dark:text-white">{{ observationTitle(o) }}</p>
                <p class="truncate text-xs text-slate-500 dark:text-slate-400">
                  {{ patientNames.nameOf(observationPatientId(o)) ?? `Patient ${shortId(observationPatientId(o))}` }}
                  · {{ formatDateTime(o.effectiveDateTime) }}
                </p>
              </div>
              <div class="flex items-center gap-3">
                <span class="font-mono text-sm text-slate-700 tabular-nums dark:text-slate-200">{{ observationValue(o) || 'No value' }}</span>
                <StatusBadge v-if="isAbnormal(o)" tone="danger">Abnormal</StatusBadge>
                <ObservationStatus :status="o.status" />
              </div>
            </RouterLink>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>
