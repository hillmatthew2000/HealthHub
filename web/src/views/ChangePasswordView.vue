<script setup lang="ts">
import { computed, nextTick, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, ApiError } from '@/api'
import { useToastStore } from '@/stores/toast'
import { type Errors, byteLength, charLength, fieldError, hasErrors, passwordProblem, PASSWORD_MAX, PASSWORD_MIN } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'

const router = useRouter()
const toast = useToastStore()

const form = reactive({ currentPassword: '', newPassword: '', confirm: '' })
const errors = ref<Errors>({})
const submitError = ref<unknown>(null)
const saving = ref(false)

const length = computed(() => charLength(form.newPassword))
const meter = computed(() => {
  const n = length.value
  if (!n) return { pct: 0, label: '', cls: '' }
  if (passwordProblem(form.newPassword)) return { pct: Math.min(100, (n / PASSWORD_MIN) * 40), label: 'Too short', cls: 'bg-red-500' }
  const variety = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((r) => r.test(form.newPassword)).length
  if (n >= 20 || (n >= 16 && variety >= 3)) return { pct: 100, label: 'Strong', cls: 'bg-emerald-500' }
  if (n >= 14 || variety >= 3) return { pct: 70, label: 'Good', cls: 'bg-brand-500' }
  return { pct: 50, label: 'Fair', cls: 'bg-amber-500' }
})

function validate(): Errors {
  const e: Errors = {}
  if (!form.currentPassword) e.currentPassword = 'is required'
  const p = passwordProblem(form.newPassword)
  if (p) e.newPassword = p
  else if (form.newPassword === form.currentPassword) e.newPassword = 'must differ from the current password'
  if (!p && form.confirm !== form.newPassword) e.confirm = 'does not match the new password'
  return e
}

async function submit(): Promise<void> {
  submitError.value = null
  errors.value = validate()
  if (hasErrors(errors.value)) {
    await nextTick()
    document.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
    return
  }
  saving.value = true
  try {
    await api.auth.changePassword({ currentPassword: form.currentPassword, newPassword: form.newPassword })
    form.currentPassword = form.newPassword = form.confirm = ''
    toast.success('Password changed.')
    await router.push('/profile')
  } catch (e) {
    submitError.value = e
    if (e instanceof ApiError && e.problem.errors?.length) errors.value = e.fieldErrors
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="max-w-xl">
    <RouterLink to="/profile" class="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white">
      <AppIcon name="arrow-left" class="size-4" /> Profile
    </RouterLink>
    <PageHeader title="Change password" :description="`Choose a new password of ${PASSWORD_MIN} to ${PASSWORD_MAX} characters. A long passphrase works well.`" />

    <form class="card space-y-4 p-5" novalidate @submit.prevent="submit">
      <ProblemAlert v-if="submitError" :error="submitError" title="Could not change the password" />
      <FormField v-slot="{ attrs }" label="Current password" :error="fieldError(errors, 'currentPassword')" required>
        <input v-bind="attrs" v-model="form.currentPassword" type="password" class="input" autocomplete="current-password" maxlength="1024" />
      </FormField>
      <FormField
        v-slot="{ attrs }"
        label="New password"
        :error="fieldError(errors, 'newPassword')"
        :hint="`${length} of ${PASSWORD_MIN} to ${PASSWORD_MAX} characters${byteLength(form.newPassword) > length ? `, ${byteLength(form.newPassword)} bytes` : ''}.`"
        required
      >
        <input v-bind="attrs" v-model="form.newPassword" type="password" class="input" autocomplete="new-password" />
      </FormField>
      <div v-if="meter.label" class="flex items-center gap-3" aria-hidden="true">
        <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800">
          <div :class="['h-full rounded-full transition-all', meter.cls]" :style="{ width: `${meter.pct}%` }" />
        </div>
        <span class="w-16 text-right text-xs text-slate-500">{{ meter.label }}</span>
      </div>
      <FormField v-slot="{ attrs }" label="Confirm new password" :error="fieldError(errors, 'confirm')" required>
        <input v-bind="attrs" v-model="form.confirm" type="password" class="input" autocomplete="new-password" />
      </FormField>
      <div class="flex justify-end gap-2 border-t border-slate-200 pt-4 dark:border-slate-800">
        <RouterLink to="/profile" class="btn btn-secondary">Cancel</RouterLink>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Saving...' : 'Change password' }}</button>
      </div>
    </form>
  </div>
</template>
