<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, ApiError } from '@/api'
import type { Role, UpdateUserRequest, UserInfo } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { formatDateTime, userName } from '@/lib/format'
import { type Errors, fieldError } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import RolePicker from '@/components/RolePicker.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const props = defineProps<{ id: string }>()
const auth = useAuthStore()
const toast = useToastStore()

const user = ref<UserInfo | null>(null)
const roles = ref<Role[]>([])
const active = ref(true)
const loadError = ref<unknown>(null)
const submitError = ref<unknown>(null)
const errors = ref<Errors>({})
const saving = ref(false)

const isSelf = computed(() => user.value?.id === auth.user?.id)
const sameRoles = (a: Role[], b: Role[]) => [...a].sort().join() === [...b].sort().join()
const dirty = computed(() => !!user.value && (!sameRoles(roles.value, user.value.roles) || active.value !== user.value.active))

function reset(u: UserInfo): void {
  user.value = u
  roles.value = [...u.roles]
  active.value = u.active
}

watch(
  () => props.id,
  async (id) => {
    loadError.value = null
    try {
      reset(await api.users.get(id))
    } catch (e) {
      loadError.value = e
    }
  },
  { immediate: true },
)

async function save(): Promise<void> {
  if (!user.value) return
  submitError.value = null
  errors.value = {}
  const body: UpdateUserRequest = {}
  if (!sameRoles(roles.value, user.value.roles)) body.roles = roles.value
  if (active.value !== user.value.active) body.active = active.value
  saving.value = true
  try {
    const updated = await api.users.update(user.value.id, body)
    reset(updated)
    toast.success('User updated. Changes apply when the user next signs in or refreshes their session.')
    if (updated.id === auth.user?.id) auth.setUser({ ...auth.user, ...updated })
  } catch (e) {
    submitError.value = e
    if (e instanceof ApiError && e.problem.errors?.length) errors.value = e.fieldErrors
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="max-w-3xl">
    <RouterLink to="/users" class="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white">
      <AppIcon name="arrow-left" class="size-4" /> Users
    </RouterLink>

    <ProblemAlert v-if="loadError" :error="loadError" />

    <template v-else-if="user">
      <PageHeader :title="userName(user)" :description="user.email">
        <template #actions>
          <StatusBadge :tone="user.active ? 'success' : 'neutral'" dot>{{ user.active ? 'Active' : 'Deactivated' }}</StatusBadge>
        </template>
      </PageHeader>

      <form class="card space-y-5 p-5" novalidate @submit.prevent="save">
        <ProblemAlert v-if="submitError" :error="submitError" title="Could not update the user" />
        <p
          v-if="isSelf"
          class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
        >
          This is your own account. Removing your administrator role or deactivating yourself takes effect when your session
          refreshes. The API keeps at least one active administrator.
        </p>

        <RolePicker v-model="roles" :error="fieldError(errors, 'roles')" />

        <div class="flex items-start gap-3 rounded-lg border border-slate-200 p-3 dark:border-slate-700">
          <input id="user-active" v-model="active" type="checkbox" class="mt-0.5 size-4 accent-brand-600" />
          <label for="user-active" class="text-sm">
            <span class="block font-medium">Account active</span>
            <span class="block text-xs text-slate-500 dark:text-slate-400">Deactivated accounts cannot sign in or refresh their session.</span>
          </label>
        </div>

        <dl class="grid gap-3 text-sm sm:grid-cols-3">
          <div>
            <dt class="text-xs text-slate-500 uppercase">Created</dt>
            <dd>{{ formatDateTime(user.createdAt) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500 uppercase">Last sign in</dt>
            <dd>{{ user.lastLoginAt ? formatDateTime(user.lastLoginAt) : 'Never' }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500 uppercase">User ID</dt>
            <dd class="font-mono text-xs break-all select-all">{{ user.id }}</dd>
          </div>
        </dl>

        <div class="flex justify-end gap-2 border-t border-slate-200 pt-4 dark:border-slate-800">
          <button type="button" class="btn btn-secondary" :disabled="!dirty || saving" @click="reset(user)">Reset</button>
          <button type="submit" class="btn btn-primary" :disabled="!dirty || saving">{{ saving ? 'Saving...' : 'Save changes' }}</button>
        </div>
      </form>
    </template>

    <div v-else class="card h-72 animate-pulse" aria-busy="true" />
  </div>
</template>
