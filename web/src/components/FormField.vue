<script setup lang="ts">
import { computed, useId } from 'vue'

// Labels a control and wires up its hint and error for assistive tech. The
// default slot receives the attributes to bind onto the control.
const props = defineProps<{ label: string; error?: string; hint?: string; required?: boolean; id?: string }>()

const autoId = useId()
const controlId = computed(() => props.id ?? `f-${autoId}`)
const hintId = computed(() => `${controlId.value}-hint`)
const errorId = computed(() => `${controlId.value}-error`)
const describedBy = computed(() => [props.hint ? hintId.value : '', props.error ? errorId.value : ''].filter(Boolean).join(' ') || undefined)
const attrs = computed(() => ({
  id: controlId.value,
  'aria-invalid': props.error ? (true as const) : undefined,
  'aria-describedby': describedBy.value,
  'aria-required': props.required ? (true as const) : undefined,
}))
</script>

<template>
  <div>
    <label :for="controlId" class="label">
      {{ label }}<span v-if="required" class="text-red-600 dark:text-red-400" aria-hidden="true"> *</span>
    </label>
    <slot :attrs="attrs" />
    <p v-if="hint && !error" :id="hintId" class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ hint }}</p>
    <p v-if="error" :id="errorId" class="mt-1 text-xs font-medium text-red-600 dark:text-red-400">{{ label }} {{ error }}</p>
  </div>
</template>
