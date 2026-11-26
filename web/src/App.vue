<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import ToastHost from '@/components/ToastHost.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

// Any way the session ends (sign out, expiry, a 401 from the API) lands on
// the sign-in page, remembering where the user was.
watch(
  () => auth.token,
  (token) => {
    if (!token && !route.meta.public) {
      const redirect = route.fullPath !== '/' ? route.fullPath : undefined
      void router.replace({ name: 'login', query: redirect && auth.logoutReason !== 'manual' ? { redirect } : {} })
    }
  },
)

let last = 0
function activity(): void {
  const now = Date.now()
  if (now - last > 15_000) {
    last = now
    auth.markActivity()
  }
}

const events = ['pointerdown', 'keydown', 'scroll'] as const
onMounted(() => events.forEach((e) => window.addEventListener(e, activity, { passive: true })))
onBeforeUnmount(() => events.forEach((e) => window.removeEventListener(e, activity)))
</script>

<template>
  <RouterView />
  <ToastHost />
</template>
