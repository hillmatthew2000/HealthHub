<script setup lang="ts">
import type { Role } from '@/api/types'
import { ROLES } from '@/api/types'
import { ROLE_DESCRIPTIONS, ROLE_LABELS } from '@/lib/permissions'

const model = defineModel<Role[]>({ required: true })
defineProps<{ error?: string }>()

function toggle(r: Role, on: boolean): void {
  model.value = on ? [...new Set([...model.value, r])] : model.value.filter((x) => x !== r)
}
</script>

<template>
  <fieldset :aria-invalid="error ? 'true' : undefined" aria-describedby="roles-error">
    <legend class="label">Roles</legend>
    <div class="grid gap-2 sm:grid-cols-2">
      <label
        v-for="r in ROLES"
        :key="r"
        :class="[
          'flex cursor-pointer gap-3 rounded-lg border p-3 transition-colors',
          model.includes(r)
            ? 'border-brand-500 bg-brand-50 dark:border-brand-600 dark:bg-brand-950/40'
            : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800/50',
        ]"
      >
        <input
          type="checkbox"
          class="mt-0.5 size-4 accent-brand-600"
          :checked="model.includes(r)"
          @change="toggle(r, ($event.target as HTMLInputElement).checked)"
        />
        <span>
          <span class="block text-sm font-medium">{{ ROLE_LABELS[r] }}</span>
          <span class="block text-xs text-slate-500 dark:text-slate-400">{{ ROLE_DESCRIPTIONS[r] }}</span>
        </span>
      </label>
    </div>
    <p v-if="error" id="roles-error" class="mt-1 text-xs font-medium text-red-600 dark:text-red-400">Roles {{ error }}</p>
  </fieldset>
</template>
