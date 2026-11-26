<script setup lang="ts">
import { useToastStore } from '@/stores/toast'
import AppIcon from './AppIcon.vue'

const toasts = useToastStore()
</script>

<template>
  <div aria-live="polite" class="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4 sm:items-end">
    <TransitionGroup
      enter-from-class="translate-y-2 opacity-0"
      enter-active-class="transition duration-200 ease-out"
      leave-to-class="opacity-0"
      leave-active-class="transition duration-150 ease-in"
    >
      <div
        v-for="t in toasts.toasts"
        :key="t.id"
        :role="t.kind === 'error' ? 'alert' : 'status'"
        class="pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-lg border border-slate-200 bg-white p-3 text-sm shadow-lg dark:border-slate-700 dark:bg-slate-800"
      >
        <AppIcon
          :name="t.kind === 'success' ? 'check' : t.kind === 'error' ? 'alert' : 'info'"
          :class="`size-5 shrink-0 ${t.kind === 'success' ? 'text-emerald-500' : t.kind === 'error' ? 'text-red-500' : 'text-sky-500'}`"
        />
        <p class="flex-1 text-slate-700 dark:text-slate-200">{{ t.message }}</p>
        <button type="button" class="rounded p-0.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200" aria-label="Dismiss" @click="toasts.dismiss(t.id)">
          <AppIcon name="x" class="size-4" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
