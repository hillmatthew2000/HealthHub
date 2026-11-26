<script setup lang="ts">
import { computed } from 'vue'
import StatusBadge, { type Tone } from './StatusBadge.vue'
import { titleCase } from '@/lib/format'

const props = defineProps<{ status: string }>()

const tones: Record<string, Tone> = {
  final: 'success',
  amended: 'info',
  corrected: 'info',
  preliminary: 'warning',
  registered: 'neutral',
  cancelled: 'neutral',
  'entered-in-error': 'danger',
  unknown: 'neutral',
}
const tone = computed(() => tones[props.status] ?? 'neutral')
</script>

<template>
  <StatusBadge :tone="tone" dot>{{ titleCase(status) }}</StatusBadge>
</template>
