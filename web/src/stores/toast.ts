import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Toast {
  id: number
  kind: 'success' | 'error' | 'info'
  message: string
}

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([])
  let next = 1

  function dismiss(id: number): void {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  function push(kind: Toast['kind'], message: string, timeoutMs = 5000): void {
    const id = next++
    toasts.value.push({ id, kind, message })
    setTimeout(() => dismiss(id), timeoutMs)
  }

  return {
    toasts,
    dismiss,
    success: (m: string) => push('success', m),
    error: (m: string) => push('error', m, 8000),
    info: (m: string) => push('info', m),
  }
})
