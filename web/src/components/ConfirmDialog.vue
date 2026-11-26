<script setup lang="ts">
import { ref, watch } from 'vue'
import ProblemAlert from './ProblemAlert.vue'

// A modal confirmation built on <dialog>, which traps focus, closes on Escape
// and returns focus to the opener.
const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    confirmLabel?: string
    danger?: boolean
    busy?: boolean
    error?: unknown
  }>(),
  { confirmLabel: 'Confirm', danger: false, busy: false, error: undefined },
)
const emit = defineEmits<{ confirm: []; cancel: [] }>()

const dialog = ref<HTMLDialogElement | null>(null)

watch(
  () => props.open,
  (open) => {
    const d = dialog.value
    if (!d) return
    if (open && !d.open) d.showModal()
    if (!open && d.open) d.close()
  },
  { flush: 'post' },
)

function onCancel(e: Event): void {
  e.preventDefault()
  if (!props.busy) emit('cancel')
}
</script>

<template>
  <dialog
    ref="dialog"
    aria-labelledby="confirm-title"
    class="m-auto w-[min(28rem,calc(100vw-2rem))] rounded-xl border border-slate-200 bg-white p-0 text-slate-900 shadow-xl backdrop:bg-slate-900/50 backdrop:backdrop-blur-sm dark:border-slate-800 dark:bg-slate-900 dark:text-slate-100"
    @cancel="onCancel"
  >
    <div class="space-y-4 p-6">
      <h2 id="confirm-title" class="text-lg font-semibold">{{ title }}</h2>
      <div class="text-sm text-slate-600 dark:text-slate-300"><slot /></div>
      <ProblemAlert :error="error" />
    </div>
    <div class="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-3 dark:border-slate-800 dark:bg-slate-900/60">
      <button type="button" class="btn btn-secondary" :disabled="busy" @click="emit('cancel')">Cancel</button>
      <button type="button" :class="['btn', danger ? 'btn-danger' : 'btn-primary']" :disabled="busy" @click="emit('confirm')">
        {{ busy ? 'Working...' : confirmLabel }}
      </button>
    </div>
  </dialog>
</template>
