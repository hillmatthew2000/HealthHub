<script setup lang="ts">
import type { Observation } from '@/api/types'
import { categoryCodes, formatDateTime, isAbnormal, observationPatientId, observationTitle, observationValue, shortId } from '@/lib/format'
import ObservationStatus from './ObservationStatus.vue'
import StatusBadge from './StatusBadge.vue'

defineProps<{
  observations: Observation[]
  loading?: boolean
  showPatient?: boolean
  patientName?: (id: string | undefined) => string | undefined
}>()
</script>

<template>
  <div class="overflow-x-auto">
    <table class="table-base" :aria-busy="loading">
      <caption class="sr-only">Observations</caption>
      <thead>
        <tr>
          <th scope="col">Observation</th>
          <th v-if="showPatient" scope="col">Patient</th>
          <th scope="col">Value</th>
          <th scope="col" class="hidden lg:table-cell">Category</th>
          <th scope="col">Effective</th>
          <th scope="col">Status</th>
        </tr>
      </thead>
      <tbody>
        <template v-if="loading && observations.length === 0">
          <tr v-for="i in 5" :key="i">
            <td :colspan="showPatient ? 6 : 5"><div class="h-4 animate-pulse rounded bg-slate-200 dark:bg-slate-800" /></td>
          </tr>
        </template>
        <tr v-for="o in observations" :key="o.id" class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
          <td>
            <RouterLink :to="`/observations/${o.id}`" class="link">{{ observationTitle(o) }}</RouterLink>
            <p v-if="o.code.coding?.[0]?.code" class="font-mono text-xs text-slate-400">{{ o.code.coding[0].code }}</p>
          </td>
          <td v-if="showPatient">
            <RouterLink v-if="observationPatientId(o)" :to="`/patients/${observationPatientId(o)}`" class="link">
              {{ patientName?.(observationPatientId(o)) ?? `Patient ${shortId(observationPatientId(o))}` }}
            </RouterLink>
          </td>
          <td class="whitespace-nowrap">
            <span class="font-mono tabular-nums">{{ observationValue(o) || '' }}</span>
            <StatusBadge v-if="isAbnormal(o)" tone="danger" class="ml-2">Abnormal</StatusBadge>
          </td>
          <td class="hidden lg:table-cell">{{ categoryCodes(o).join(', ') }}</td>
          <td class="whitespace-nowrap">{{ formatDateTime(o.effectiveDateTime) }}</td>
          <td><ObservationStatus :status="o.status" /></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
