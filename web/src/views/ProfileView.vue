<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/api'
import type { UserInfo } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime, initials, userName } from '@/lib/format'
import { ROLE_DESCRIPTIONS, ROLE_LABELS } from '@/lib/permissions'
import AppIcon from '@/components/AppIcon.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const auth = useAuthStore()
const profile = ref<UserInfo | null>(auth.user)
const error = ref<unknown>(null)

onMounted(async () => {
  try {
    profile.value = await api.auth.profile()
    auth.setUser(profile.value)
  } catch (e) {
    error.value = e
  }
})
</script>

<template>
  <div class="max-w-3xl">
    <PageHeader title="Profile" description="Your account details. An administrator manages your roles.">
      <template #actions>
        <RouterLink to="/profile/password" class="btn btn-secondary"><AppIcon name="key" class="size-4" /> Change password</RouterLink>
      </template>
    </PageHeader>

    <ProblemAlert v-if="error" :error="error" class="mb-4" />

    <section v-if="profile" class="card overflow-hidden">
      <div class="flex items-center gap-4 border-b border-slate-200 p-5 dark:border-slate-800">
        <span class="flex size-14 items-center justify-center rounded-full bg-brand-100 text-lg font-semibold text-brand-800 dark:bg-brand-900 dark:text-brand-200" aria-hidden="true">
          {{ initials(userName(profile)) }}
        </span>
        <div>
          <p class="text-lg font-semibold">{{ userName(profile) }}</p>
          <p class="text-sm text-slate-500 dark:text-slate-400">{{ profile.email }}</p>
        </div>
        <StatusBadge class="ml-auto" :tone="profile.active ? 'success' : 'neutral'" dot>{{ profile.active ? 'Active' : 'Deactivated' }}</StatusBadge>
      </div>
      <dl class="grid gap-5 p-5 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Roles</dt>
          <dd class="mt-2 space-y-2">
            <div v-for="r in profile.roles" :key="r" class="flex flex-wrap items-baseline gap-2">
              <StatusBadge :tone="r === 'admin' ? 'brand' : 'info'">{{ ROLE_LABELS[r] ?? r }}</StatusBadge>
              <span class="text-sm text-slate-600 dark:text-slate-300">{{ ROLE_DESCRIPTIONS[r] }}</span>
            </div>
            <p v-if="profile.roles.length === 0" class="text-sm text-slate-500">No roles assigned. Ask an administrator for access.</p>
          </dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Last sign in</dt>
          <dd class="mt-1 text-sm">{{ profile.lastLoginAt ? formatDateTime(profile.lastLoginAt) : 'Never' }}</dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Account created</dt>
          <dd class="mt-1 text-sm">{{ formatDateTime(profile.createdAt) }}</dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">Session expires</dt>
          <dd class="mt-1 text-sm">
            {{ auth.expiresAt ? formatDateTime(new Date(auth.expiresAt).toISOString()) : '' }}
            <span class="block text-xs text-slate-500">Renewed automatically while you are active.</span>
          </dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-slate-500 uppercase dark:text-slate-400">User ID</dt>
          <dd class="mt-1 font-mono text-xs break-all select-all">{{ profile.id }}</dd>
        </div>
      </dl>
    </section>
  </div>
</template>
