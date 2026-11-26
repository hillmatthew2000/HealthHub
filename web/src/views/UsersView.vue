<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { Page, UserInfo } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime, initials, userName } from '@/lib/format'
import { compactQuery, qint } from '@/lib/query'
import AppIcon from '@/components/AppIcon.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import RoleBadges from '@/components/RoleBadges.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const page = ref<Page<UserInfo> | null>(null)
const loading = ref(false)
const error = ref<unknown>(null)
const pageNo = computed(() => qint(route.query, 'page', 1))
const limit = computed(() => qint(route.query, 'limit', 20, 1, 100))

async function load(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    page.value = await api.users.list({ page: pageNo.value, limit: limit.value })
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

function go(p: number, l = limit.value): void {
  void router.replace({ query: compactQuery({ page: p > 1 ? p : undefined, limit: l === 20 ? undefined : l }) })
}

watch(() => route.query, () => void load(), { immediate: true })
</script>

<template>
  <div>
    <PageHeader title="Users" description="Staff accounts and their roles, sorted by email.">
      <template #actions>
        <RouterLink v-if="auth.can('users', 'create')" to="/users/new" class="btn btn-primary">
          <AppIcon name="plus" class="size-4" /> New user
        </RouterLink>
      </template>
    </PageHeader>

    <ProblemAlert v-if="error" :error="error" class="mb-4" />

    <div class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="table-base" :aria-busy="loading">
          <caption class="sr-only">Users</caption>
          <thead>
            <tr>
              <th scope="col">User</th>
              <th scope="col">Roles</th>
              <th scope="col">Status</th>
              <th scope="col" class="hidden md:table-cell">Last sign in</th>
              <th scope="col"><span class="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loading && !page">
              <tr v-for="i in 4" :key="i">
                <td colspan="5"><div class="h-4 animate-pulse rounded bg-slate-200 dark:bg-slate-800" /></td>
              </tr>
            </template>
            <tr v-for="u in page?.data ?? []" :key="u.id" class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
              <td>
                <div class="flex items-center gap-3">
                  <span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600 dark:bg-slate-800 dark:text-slate-300" aria-hidden="true">
                    {{ initials(userName(u)) }}
                  </span>
                  <div class="min-w-0">
                    <p class="truncate font-medium text-slate-900 dark:text-white">
                      {{ userName(u) }}
                      <span v-if="u.id === auth.user?.id" class="ml-1 text-xs font-normal text-slate-400">(you)</span>
                    </p>
                    <p class="truncate text-xs text-slate-500">{{ u.email }}</p>
                  </div>
                </div>
              </td>
              <td><RoleBadges :roles="u.roles" /></td>
              <td>
                <StatusBadge :tone="u.active ? 'success' : 'neutral'" dot>{{ u.active ? 'Active' : 'Deactivated' }}</StatusBadge>
              </td>
              <td class="hidden whitespace-nowrap md:table-cell">{{ u.lastLoginAt ? formatDateTime(u.lastLoginAt) : 'Never' }}</td>
              <td class="text-right">
                <RouterLink v-if="auth.can('users', 'update')" :to="`/users/${u.id}`" class="btn btn-ghost btn-sm" :aria-label="`Edit ${userName(u)}`">
                  <AppIcon name="pencil" class="size-4" /> Edit
                </RouterLink>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-if="page && page.data.length === 0" title="No users" icon="users" />
      <PaginationBar
        v-if="page && page.total > 0"
        :page="page.page"
        :limit="page.limit"
        :total="page.total"
        :total-pages="page.totalPages"
        @update:page="(p) => go(p)"
        @update:limit="(l) => go(1, l)"
      />
    </div>
  </div>
</template>
