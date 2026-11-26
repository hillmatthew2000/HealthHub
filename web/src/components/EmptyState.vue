<script setup lang="ts">
import AppIcon, { type IconName } from './AppIcon.vue'

defineProps<{ title: string; icon?: IconName }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center px-6 py-14 text-center">
    <div class="mb-3 rounded-full bg-slate-100 p-3 text-slate-500 dark:bg-slate-800 dark:text-slate-400">
      <AppIcon :name="icon ?? 'search'" class="size-6" />
    </div>
    <p class="font-medium text-slate-900 dark:text-white">{{ title }}</p>
    <div class="mt-1 max-w-md text-sm text-slate-500 dark:text-slate-400"><slot /></div>
    <div v-if="$slots.actions" class="mt-4"><slot name="actions" /></div>
  </div>
</template>
