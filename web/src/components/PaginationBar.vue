<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ page: number; limit: number; total: number; totalPages: number; limits?: number[] }>()
const emit = defineEmits<{ 'update:page': [number]; 'update:limit': [number] }>()

const first = computed(() => (props.total === 0 ? 0 : (props.page - 1) * props.limit + 1))
const last = computed(() => Math.min(props.page * props.limit, props.total))
const sizes = computed(() => props.limits ?? [10, 20, 50, 100])
</script>

<template>
  <nav
    aria-label="Pagination"
    class="flex flex-col gap-3 border-t border-slate-200 px-4 py-3 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between dark:border-slate-800 dark:text-slate-400"
  >
    <p aria-live="polite">
      <template v-if="total > 0">
        Showing <span class="font-medium text-slate-900 dark:text-white">{{ first }}</span> to
        <span class="font-medium text-slate-900 dark:text-white">{{ last }}</span> of
        <span class="font-medium text-slate-900 dark:text-white">{{ total }}</span>
      </template>
      <template v-else>No results</template>
    </p>
    <div class="flex items-center gap-3">
      <label class="flex items-center gap-2">
        <span>Rows</span>
        <select
          class="input w-auto py-1"
          :value="limit"
          @change="emit('update:limit', Number(($event.target as HTMLSelectElement).value))"
        >
          <option v-for="s in sizes" :key="s" :value="s">{{ s }}</option>
        </select>
      </label>
      <div class="flex items-center gap-1">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="page <= 1"
          aria-label="Previous page"
          @click="emit('update:page', page - 1)"
        >
          <AppIcon name="chevron-left" class="size-4" />
        </button>
        <span class="px-2 tabular-nums">Page {{ page }} of {{ Math.max(totalPages, 1) }}</span>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="page >= totalPages"
          aria-label="Next page"
          @click="emit('update:page', page + 1)"
        >
          <AppIcon name="chevron-right" class="size-4" />
        </button>
      </div>
    </div>
  </nav>
</template>
