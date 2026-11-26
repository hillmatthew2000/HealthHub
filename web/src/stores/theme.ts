import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ThemePreference = 'light' | 'dark' | 'system'

const KEY = 'healthhub.theme'

function load(): ThemePreference {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

export const useThemeStore = defineStore('theme', () => {
  const preference = ref<ThemePreference>(load())
  const media = typeof window !== 'undefined' ? window.matchMedia?.('(prefers-color-scheme: dark)') : undefined

  function isDark(): boolean {
    return preference.value === 'dark' || (preference.value === 'system' && !!media?.matches)
  }

  function apply(): void {
    document.documentElement.classList.toggle('dark', isDark())
  }

  function set(p: ThemePreference): void {
    preference.value = p
    try {
      if (p === 'system') localStorage.removeItem(KEY)
      else localStorage.setItem(KEY, p)
    } catch {
      // Not persisted; the choice lasts for this page view.
    }
    apply()
  }

  function cycle(): void {
    set(preference.value === 'light' ? 'dark' : preference.value === 'dark' ? 'system' : 'light')
  }

  media?.addEventListener?.('change', apply)

  return { preference, set, cycle, apply, isDark }
})
