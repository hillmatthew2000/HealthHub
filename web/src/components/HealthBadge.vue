<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '@/api'
import type { HealthResult } from '@/api/endpoints'

// Polls GET /health/ready and shows whether the API and its database are up.
const props = withDefaults(defineProps<{ intervalMs?: number; detailed?: boolean }>(), {
  intervalMs: 30_000,
  detailed: false,
})

const result = ref<HealthResult | null>(null)
const unreachable = ref(false)
const checkedAt = ref<Date | null>(null)
let timer: ReturnType<typeof setInterval> | undefined
let controller: AbortController | undefined

async function check(): Promise<void> {
  controller?.abort()
  controller = new AbortController()
  try {
    result.value = await api.health.ready(controller.signal)
    unreachable.value = false
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') return
    result.value = null
    unreachable.value = true
  }
  checkedAt.value = new Date()
}

const state = computed<'ok' | 'degraded' | 'down' | 'unknown'>(() => {
  if (unreachable.value) return 'down'
  if (!result.value) return 'unknown'
  return result.value.ok ? 'ok' : 'degraded'
})

const label = computed(
  () =>
    ({
      ok: 'API healthy',
      degraded: 'API not ready',
      down: 'API unreachable',
      unknown: 'Checking API',
    })[state.value],
)

const tone = computed(
  () =>
    ({
      ok: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-950/50 dark:text-emerald-300',
      degraded: 'bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-950/50 dark:text-amber-300',
      down: 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-950/50 dark:text-red-300',
      unknown: 'bg-slate-100 text-slate-600 ring-slate-500/20 dark:bg-slate-800 dark:text-slate-300',
    })[state.value],
)

const dot = computed(
  () => ({ ok: 'bg-emerald-500', degraded: 'bg-amber-500', down: 'bg-red-500', unknown: 'bg-slate-400' })[state.value],
)

const title = computed(() => {
  const parts = [label.value]
  if (result.value?.version) parts.push(`version ${result.value.version}`)
  if (result.value?.checks) parts.push(...Object.entries(result.value.checks).map(([k, v]) => `${k} ${v}`))
  if (checkedAt.value) parts.push(`checked ${checkedAt.value.toLocaleTimeString()}`)
  return parts.join(', ')
})

onMounted(() => {
  void check()
  timer = setInterval(() => void check(), props.intervalMs)
})
onBeforeUnmount(() => {
  clearInterval(timer)
  controller?.abort()
})

defineExpose({ check })
</script>

<template>
  <span
    role="status"
    :title="title"
    :class="['inline-flex items-center gap-2 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset', tone]"
  >
    <span class="relative flex size-2" aria-hidden="true">
      <span v-if="state === 'ok'" :class="['absolute inline-flex size-full animate-ping rounded-full opacity-50', dot]" />
      <span :class="['relative inline-flex size-2 rounded-full', dot]" />
    </span>
    <span>{{ label }}</span>
    <span v-if="detailed && result?.version" class="font-mono opacity-70">{{ result.version }}</span>
  </span>
</template>
