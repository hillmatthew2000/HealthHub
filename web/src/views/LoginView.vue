<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { safeRedirect } from '@/router'
import { isEmail } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import FormField from '@/components/FormField.vue'
import HealthBadge from '@/components/HealthBadge.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const form = reactive({ email: '', password: '' })
const errors = reactive<{ email?: string; password?: string }>({})
const busy = ref(false)
const error = ref<unknown>(null)

const notice = computed(() => {
  switch (auth.logoutReason) {
    case 'expired':
    case 'unauthorized':
      return 'Your session has ended. Sign in again to continue.'
    case 'idle':
      return 'You were signed out after a period of inactivity.'
    case 'manual':
      return 'You have signed out.'
    default:
      return ''
  }
})

async function submit(): Promise<void> {
  errors.email = !form.email.trim() ? 'is required' : !isEmail(form.email) ? 'must be a valid email address' : undefined
  errors.password = !form.password ? 'is required' : undefined
  if (errors.email || errors.password) return
  busy.value = true
  error.value = null
  try {
    await auth.login(form.email.trim(), form.password)
    form.password = ''
    await router.replace(safeRedirect(route.query.redirect))
  } catch (err) {
    error.value = err
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen">
    <div class="absolute top-4 right-4"><ThemeToggle /></div>

    <section
      class="relative hidden flex-1 flex-col justify-between overflow-hidden bg-linear-to-br from-brand-700 via-brand-800 to-slate-900 p-12 text-white lg:flex"
      aria-hidden="true"
    >
      <div class="flex items-center gap-2.5">
        <div class="flex size-9 items-center justify-center rounded-lg bg-white/15 ring-1 ring-white/20">
          <AppIcon name="heart" class="size-5" />
        </div>
        <span class="text-lg font-semibold">HealthHub</span>
      </div>
      <div class="max-w-md">
        <p class="text-3xl leading-tight font-semibold">Patient records and clinical observations, in one secure place.</p>
        <p class="mt-4 text-brand-100/80">
          Role-based access, encrypted storage and a complete audit trail of who viewed and changed what.
        </p>
      </div>
      <div class="absolute -right-24 -bottom-24 size-96 rounded-full bg-white/5" />
      <div class="absolute top-1/3 -right-10 size-48 rounded-full bg-white/5" />
      <p class="text-xs text-brand-100/60">Authorised staff only. Access is logged.</p>
    </section>

    <main class="flex flex-1 items-center justify-center px-4 py-12 sm:px-8">
      <div class="w-full max-w-sm">
        <div class="mb-8 lg:hidden">
          <div class="flex size-10 items-center justify-center rounded-lg bg-brand-600 text-white">
            <AppIcon name="heart" class="size-6" />
          </div>
        </div>
        <h1 id="page-title" tabindex="-1" class="text-2xl font-semibold tracking-tight focus:outline-none">Sign in to HealthHub</h1>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">Use the account your administrator created for you.</p>

        <p
          v-if="notice && !error"
          role="status"
          class="mt-6 rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-sm text-sky-800 dark:border-sky-900 dark:bg-sky-950/40 dark:text-sky-200"
        >
          {{ notice }}
        </p>

        <form class="mt-6 space-y-4" novalidate @submit.prevent="submit">
          <ProblemAlert :error="error" :title="error ? 'Sign in failed' : undefined" />
          <FormField v-slot="{ attrs }" label="Email" :error="errors.email" required>
            <input v-bind="attrs" v-model="form.email" type="email" autocomplete="username" class="input" maxlength="254" />
          </FormField>
          <FormField v-slot="{ attrs }" label="Password" :error="errors.password" required>
            <input v-bind="attrs" v-model="form.password" type="password" autocomplete="current-password" class="input" maxlength="1024" />
          </FormField>
          <button type="submit" class="btn btn-primary w-full" :disabled="busy">
            {{ busy ? 'Signing in...' : 'Sign in' }}
          </button>
        </form>

        <div class="mt-8 flex justify-center"><HealthBadge detailed /></div>
      </div>
    </main>
  </div>
</template>
