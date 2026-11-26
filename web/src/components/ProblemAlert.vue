<script setup lang="ts">
import { computed } from 'vue'
import { ApiError } from '@/api'
import AppIcon from './AppIcon.vue'

// Renders an API problem (RFC 9457): its detail, any field errors and the
// request ID to quote when reporting it.
const props = defineProps<{ error: unknown; title?: string }>()

const problem = computed(() => (props.error instanceof ApiError ? props.error.problem : null))
const heading = computed(() => props.title ?? problem.value?.title ?? 'Something went wrong')
const detail = computed(() => {
  if (problem.value) return problem.value.detail
  if (props.error instanceof Error) return props.error.message
  return typeof props.error === 'string' ? props.error : undefined
})
</script>

<template>
  <div
    v-if="error"
    role="alert"
    class="flex gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-200"
  >
    <AppIcon name="alert" class="size-5 shrink-0 text-red-500" />
    <div class="min-w-0 space-y-1">
      <p class="font-semibold">{{ heading }}</p>
      <p v-if="detail">{{ detail }}</p>
      <ul v-if="problem?.errors?.length" class="list-disc space-y-0.5 pl-5">
        <li v-for="e in problem.errors" :key="e.field + e.message">
          <code class="font-mono text-xs">{{ e.field }}</code> {{ e.message }}
        </li>
      </ul>
      <p v-if="problem?.requestId" class="text-xs text-red-700/80 dark:text-red-300/70">
        Request ID <code class="font-mono select-all">{{ problem.requestId }}</code>
      </p>
    </div>
  </div>
</template>
