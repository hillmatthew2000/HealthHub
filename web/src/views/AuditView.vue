<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { AuditEvent, AuditQuery, AuditResourceType, Page, UserInfo } from '@/api/types'
import { AUDIT_RESOURCE_TYPES } from '@/api/types'
import { formatDateTime, shortId, titleCase, userName } from '@/lib/format'
import { compactQuery, qint, qs } from '@/lib/query'
import { isIsoDate, isUuid } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import StatusBadge, { type Tone } from '@/components/StatusBadge.vue'

const route = useRoute()
const router = useRouter()

const keys = ['actor', 'resource', 'resourceType', 'from', 'to'] as const
const filters = reactive<Record<(typeof keys)[number], string>>({ actor: '', resource: '', resourceType: '', from: '', to: '' })
const localErrors = reactive<Record<string, string>>({})
const page = ref<Page<AuditEvent> | null>(null)
const loading = ref(false)
const error = ref<unknown>(null)
const users = ref<Map<string, UserInfo>>(new Map())

const pageNo = computed(() => qint(route.query, 'page', 1))
const limit = computed(() => qint(route.query, 'limit', 50, 1, 100))
const hasFilters = computed(() => keys.some((k) => qs(route.query, k)))

const actionTones: Record<string, Tone> = { create: 'success', read: 'info', list: 'neutral', update: 'warning', delete: 'danger' }

async function loadUsers(): Promise<void> {
  try {
    const p = await api.users.list({ limit: 100 })
    users.value = new Map(p.data.map((u) => [u.id, u]))
  } catch {
    // Names are a convenience; IDs still show.
  }
}

async function load(): Promise<void> {
  loading.value = true
  error.value = null
  const rt = qs(route.query, 'resourceType')
  const q: AuditQuery = {
    page: pageNo.value,
    limit: limit.value,
    actor: qs(route.query, 'actor') || undefined,
    resource: qs(route.query, 'resource') || undefined,
    resourceType: (AUDIT_RESOURCE_TYPES as readonly string[]).includes(rt) ? (rt as AuditResourceType) : undefined,
    from: qs(route.query, 'from') || undefined,
    to: qs(route.query, 'to') || undefined,
  }
  try {
    page.value = await api.audit.list(q)
  } catch (e) {
    error.value = e
    page.value = null
  } finally {
    loading.value = false
  }
}

function validate(): boolean {
  for (const k of Object.keys(localErrors)) delete localErrors[k]
  if (filters.actor && !isUuid(filters.actor)) localErrors.actor = 'must be a user ID (UUID)'
  if (filters.resource && !isUuid(filters.resource)) localErrors.resource = 'must be a record ID (UUID)'
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
      limit: limit.value === 50 ? undefined : limit.value,
      ...extra,
    }),
  })
}

function filterBy(key: 'actor' | 'resource', value: string | undefined): void {
  if (!value) return
  filters[key] = value
  apply()
}

function clear(): void {
  for (const k of keys) filters[k] = ''
  apply()
}

function resourceLink(type: string, id: string): string | undefined {
  if (type === 'patient') return `/patients/${id}`
  if (type === 'observation') return `/observations/${id}`
  if (type === 'user') return `/users/${id}`
  return undefined
}

watch(
  () => route.query,
  () => {
    for (const k of keys) filters[k] = qs(route.query, k)
    void load()
  },
  { immediate: true },
)
onMounted(() => void loadUsers())
</script>

<template>
  <div>
    <PageHeader title="Audit events" description="Who created, viewed, changed or deleted which records. Newest first; no PHI is stored here." />

    <form class="card mb-4 p-4" role="search" aria-label="Filter audit events" @submit.prevent="apply()">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <div>
          <label for="a-actor" class="label">Actor</label>
          <select v-if="users.size" id="a-actor" v-model="filters.actor" class="input">
            <option value="">Anyone</option>
            <option v-for="u in users.values()" :key="u.id" :value="u.id">{{ userName(u) }}</option>
          </select>
          <input v-else id="a-actor" v-model="filters.actor" class="input font-mono" placeholder="User ID" :aria-invalid="localErrors.actor ? 'true' : undefined" />
          <p v-if="localErrors.actor" class="mt-1 text-xs text-red-600">Actor {{ localErrors.actor }}</p>
        </div>
        <div>
          <label for="a-type" class="label">Resource type</label>
          <select id="a-type" v-model="filters.resourceType" class="input">
            <option value="">Any</option>
            <option v-for="t in AUDIT_RESOURCE_TYPES" :key="t" :value="t">{{ titleCase(t) }}</option>
          </select>
        </div>
        <div>
          <label for="a-resource" class="label">Record ID</label>
          <input id="a-resource" v-model="filters.resource" class="input font-mono" placeholder="UUID" :aria-invalid="localErrors.resource ? 'true' : undefined" />
          <p v-if="localErrors.resource" class="mt-1 text-xs text-red-600">Record ID {{ localErrors.resource }}</p>
        </div>
        <div>
          <label for="a-from" class="label">From</label>
          <input id="a-from" v-model="filters.from" type="date" class="input" :aria-invalid="localErrors.from ? 'true' : undefined" />
          <p v-if="localErrors.from" class="mt-1 text-xs text-red-600">From {{ localErrors.from }}</p>
        </div>
        <div>
          <label for="a-to" class="label">To</label>
          <input id="a-to" v-model="filters.to" type="date" class="input" :aria-invalid="localErrors.to ? 'true' : undefined" />
        </div>
      </div>
      <div class="mt-3 flex justify-end gap-2">
        <button v-if="hasFilters" type="button" class="btn btn-ghost" @click="clear">Clear</button>
        <button type="submit" class="btn btn-primary"><AppIcon name="filter" class="size-4" /> Apply filters</button>
      </div>
    </form>

    <ProblemAlert v-if="error" :error="error" class="mb-4" />

    <div class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="table-base" :aria-busy="loading">
          <caption class="sr-only">Audit events</caption>
          <thead>
            <tr>
              <th scope="col">When</th>
              <th scope="col">Actor</th>
              <th scope="col">Action</th>
              <th scope="col">Records</th>
              <th scope="col" class="hidden lg:table-cell">Client</th>
              <th scope="col" class="hidden xl:table-cell">Request</th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loading && !page">
              <tr v-for="i in 6" :key="i">
                <td colspan="6"><div class="h-4 animate-pulse rounded bg-slate-200 dark:bg-slate-800" /></td>
              </tr>
            </template>
            <tr v-for="e in page?.data ?? []" :key="e.id">
              <td class="whitespace-nowrap tabular-nums">{{ formatDateTime(e.occurredAt) }}</td>
              <td>
                <button v-if="e.actorId" type="button" class="link text-left" :title="`Filter by ${e.actorId}`" @click="filterBy('actor', e.actorId)">
                  {{ users.get(e.actorId) ? userName(users.get(e.actorId)) : shortId(e.actorId) }}
                </button>
                <span v-else class="text-slate-400">Anonymous</span>
              </td>
              <td class="whitespace-nowrap">
                <StatusBadge :tone="actionTones[e.action] ?? 'neutral'">{{ titleCase(e.action) }}</StatusBadge>
                <span class="ml-2 text-slate-500">{{ e.resourceType }}</span>
              </td>
              <td>
                <span v-if="e.resourceIds.length === 0" class="text-slate-400">None</span>
                <span v-else class="flex flex-wrap gap-1">
                  <template v-for="id in e.resourceIds.slice(0, 3)" :key="id">
                    <RouterLink
                      v-if="resourceLink(e.resourceType, id) && e.action !== 'delete'"
                      :to="resourceLink(e.resourceType, id)!"
                      class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700"
                      :title="id"
                    >
                      {{ shortId(id) }}
                    </RouterLink>
                    <span v-else class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs dark:bg-slate-800" :title="id">{{ shortId(id) }}</span>
                  </template>
                  <span v-if="e.resourceIds.length > 3" class="text-xs text-slate-500">+{{ e.resourceIds.length - 3 }} more</span>
                </span>
              </td>
              <td class="hidden font-mono text-xs lg:table-cell">{{ e.clientIp }}</td>
              <td class="hidden font-mono text-xs xl:table-cell" :title="e.requestId">{{ shortId(e.requestId) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-if="page && page.data.length === 0" title="No audit events" icon="shield">
        <template v-if="hasFilters">No events match these filters.</template>
      </EmptyState>
      <PaginationBar
        v-if="page && page.total > 0"
        :page="page.page"
        :limit="page.limit"
        :total="page.total"
        :total-pages="page.totalPages"
        :limits="[25, 50, 100]"
        @update:page="(p) => apply({ page: p > 1 ? p : undefined })"
        @update:limit="(l) => apply({ limit: l === 50 ? undefined : l })"
      />
    </div>
  </div>
</template>
