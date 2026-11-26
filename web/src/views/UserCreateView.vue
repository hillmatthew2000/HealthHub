<script setup lang="ts">
import { nextTick, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, ApiError } from '@/api'
import type { Role } from '@/api/types'
import { useToastStore } from '@/stores/toast'
import { type Errors, charLength, fieldError, hasErrors, isEmail, passwordProblem, PASSWORD_MAX, PASSWORD_MIN } from '@/lib/validation'
import AppIcon from '@/components/AppIcon.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProblemAlert from '@/components/ProblemAlert.vue'
import RolePicker from '@/components/RolePicker.vue'

const router = useRouter()
const toast = useToastStore()

const form = reactive({ email: '', firstName: '', lastName: '', password: '', confirm: '', roles: [] as Role[] })
const errors = ref<Errors>({})
const submitError = ref<unknown>(null)
const saving = ref(false)

function validate(): Errors {
  const e: Errors = {}
  if (!form.email.trim()) e.email = 'is required'
  else if (!isEmail(form.email) || charLength(form.email.trim()) > 254) e.email = 'must be a valid email address'
  for (const k of ['firstName', 'lastName'] as const) {
    if (!form[k].trim()) e[k] = 'is required'
    else if (charLength(form[k]) > 100) e[k] = 'must have at most 100 characters'
  }
  const pw = passwordProblem(form.password)
  if (pw) e.password = pw
  else if (form.confirm !== form.password) e.confirm = 'does not match the password'
  if (form.roles.length > 4) e.roles = 'must have at most 4 items'
  return e
}

const err = (f: string) => fieldError(errors.value, f)

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
    await api.users.create({
      email: form.email.trim(),
      firstName: form.firstName.trim(),
      lastName: form.lastName.trim(),
      password: form.password,
      roles: form.roles,
    })
    form.password = form.confirm = ''
    toast.success('User created. Share the initial password with them securely.')
    await router.push('/users')
  } catch (e) {
    submitError.value = e
    if (e instanceof ApiError && e.problem.errors?.length) errors.value = e.fieldErrors
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="max-w-3xl">
    <RouterLink to="/users" class="mb-4 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white">
      <AppIcon name="arrow-left" class="size-4" /> Users
    </RouterLink>
    <PageHeader title="New user" description="Create a staff account. The user should change this password after first sign in." />

    <form class="card space-y-5 p-5" novalidate @submit.prevent="submit">
      <ProblemAlert v-if="submitError" :error="submitError" title="Could not create the user" />
      <div class="grid gap-4 sm:grid-cols-2">
        <FormField v-slot="{ attrs }" label="First name" :error="err('firstName')" required>
          <input v-bind="attrs" v-model="form.firstName" class="input" maxlength="100" autocomplete="off" />
        </FormField>
        <FormField v-slot="{ attrs }" label="Last name" :error="err('lastName')" required>
          <input v-bind="attrs" v-model="form.lastName" class="input" maxlength="100" autocomplete="off" />
        </FormField>
        <FormField v-slot="{ attrs }" label="Email" :error="err('email')" class="sm:col-span-2" required>
          <input v-bind="attrs" v-model="form.email" type="email" class="input" maxlength="254" autocomplete="off" />
        </FormField>
        <FormField v-slot="{ attrs }" label="Initial password" :error="err('password')" :hint="`${PASSWORD_MIN} to ${PASSWORD_MAX} characters.`" required>
          <input v-bind="attrs" v-model="form.password" type="password" class="input" autocomplete="new-password" />
        </FormField>
        <FormField v-slot="{ attrs }" label="Confirm password" :error="err('confirm')" required>
          <input v-bind="attrs" v-model="form.confirm" type="password" class="input" autocomplete="new-password" />
        </FormField>
      </div>
      <RolePicker v-model="form.roles" :error="err('roles')" />
      <p v-if="form.roles.length === 0" class="text-xs text-amber-700 dark:text-amber-300">
        An account without roles can sign in but cannot see any records.
      </p>
      <div class="flex justify-end gap-2 border-t border-slate-200 pt-4 dark:border-slate-800">
        <RouterLink to="/users" class="btn btn-secondary">Cancel</RouterLink>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Creating...' : 'Create user' }}</button>
      </div>
    </form>
  </div>
</template>
