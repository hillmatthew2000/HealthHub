<script setup lang="ts">
import { computed } from 'vue'

export type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info' | 'brand'

const props = withDefaults(defineProps<{ tone?: Tone; dot?: boolean }>(), { tone: 'neutral', dot: false })

const classes: Record<Tone, string> = {
  neutral: 'bg-slate-100 text-slate-700 ring-slate-500/20 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-400/20',
  success: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-950/50 dark:text-emerald-300 dark:ring-emerald-400/30',
  warning: 'bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-950/50 dark:text-amber-300 dark:ring-amber-400/30',
  danger: 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-950/50 dark:text-red-300 dark:ring-red-400/30',
  info: 'bg-sky-50 text-sky-700 ring-sky-600/20 dark:bg-sky-950/50 dark:text-sky-300 dark:ring-sky-400/30',
  brand: 'bg-brand-50 text-brand-700 ring-brand-600/20 dark:bg-brand-950/60 dark:text-brand-300 dark:ring-brand-400/30',
}
const dots: Record<Tone, string> = {
  neutral: 'bg-slate-400',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
  info: 'bg-sky-500',
  brand: 'bg-brand-500',
}
const cls = computed(() => classes[props.tone])
</script>

<template>
  <span :class="['inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset', cls]">
    <span v-if="dot" :class="['size-1.5 rounded-full', dots[tone]]" aria-hidden="true" />
    <slot />
  </span>
</template>
