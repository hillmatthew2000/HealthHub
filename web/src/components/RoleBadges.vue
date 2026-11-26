<script setup lang="ts">
import type { Role } from '@/api/types'
import { ROLE_LABELS } from '@/lib/permissions'
import StatusBadge from './StatusBadge.vue'

defineProps<{ roles: Role[] }>()
</script>

<template>
  <span class="flex flex-wrap gap-1">
    <StatusBadge v-for="r in roles" :key="r" :tone="r === 'admin' ? 'brand' : 'info'">{{ ROLE_LABELS[r] ?? r }}</StatusBadge>
    <span v-if="roles.length === 0" class="text-xs text-slate-400">No roles</span>
  </span>
</template>
