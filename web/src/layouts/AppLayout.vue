<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import type { Permission } from '@/lib/permissions'
import { ROLE_LABELS } from '@/lib/permissions'
import { initials, userName } from '@/lib/format'
import AppIcon, { type IconName } from '@/components/AppIcon.vue'
import HealthBadge from '@/components/HealthBadge.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

const auth = useAuthStore()
const route = useRoute()
const mobileOpen = ref(false)

interface NavItem {
  to: string
  label: string
  icon: IconName
  perm?: Permission
  exact?: boolean
}

const sections: { title?: string; items: NavItem[] }[] = [
  {
    items: [
      { to: '/', label: 'Dashboard', icon: 'home', exact: true },
      { to: '/patients', label: 'Patients', icon: 'patients', perm: ['patients', 'read'] },
      { to: '/observations', label: 'Observations', icon: 'beaker', perm: ['observations', 'read'] },
    ],
  },
  {
    title: 'Administration',
    items: [
      { to: '/users', label: 'Users', icon: 'users', perm: ['users', 'read'] },
      { to: '/audit', label: 'Audit events', icon: 'shield', perm: ['audit-log', 'read'] },
    ],
  },
]

const visible = computed(() =>
  sections
    .map((s) => ({ ...s, items: s.items.filter((i) => !i.perm || auth.can(...i.perm)) }))
    .filter((s) => s.items.length > 0),
)

function isActive(item: NavItem): boolean {
  return item.exact ? route.path === item.to : route.path === item.to || route.path.startsWith(`${item.to}/`)
}

const displayName = computed(() => userName(auth.user ?? undefined))
const roleText = computed(() => auth.roles.map((r) => ROLE_LABELS[r] ?? r).join(', ') || 'No roles assigned')

// Move focus to the page heading after navigation so screen readers announce
// the new page, and close the mobile menu.
watch(
  () => route.fullPath,
  async () => {
    mobileOpen.value = false
    await nextTick()
    setTimeout(() => document.getElementById('page-title')?.focus({ preventScroll: true }), 50)
  },
)
</script>

<template>
  <div class="min-h-screen">
    <a
      href="#main"
      class="sr-only z-50 rounded-md bg-brand-600 px-3 py-2 text-white focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
    >
      Skip to content
    </a>

    <!-- Mobile backdrop -->
    <div v-if="mobileOpen" class="fixed inset-0 z-30 bg-slate-900/50 lg:hidden" aria-hidden="true" @click="mobileOpen = false" />

    <aside
      id="sidebar"
      :class="[
        'fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-slate-200 bg-white transition-transform dark:border-slate-800 dark:bg-slate-900',
        mobileOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
      ]"
    >
      <div class="flex h-16 items-center gap-2.5 border-b border-slate-200 px-5 dark:border-slate-800">
        <div class="flex size-8 items-center justify-center rounded-lg bg-brand-600 text-white shadow-sm">
          <AppIcon name="heart" class="size-5" />
        </div>
        <div>
          <p class="text-sm leading-tight font-semibold text-slate-900 dark:text-white">HealthHub</p>
          <p class="text-xs leading-tight text-slate-500 dark:text-slate-400">Clinical records</p>
        </div>
        <button type="button" class="btn btn-ghost ml-auto px-2 lg:hidden" aria-label="Close menu" @click="mobileOpen = false">
          <AppIcon name="x" />
        </button>
      </div>

      <nav aria-label="Main" class="flex-1 space-y-6 overflow-y-auto px-3 py-4">
        <div v-for="(s, i) in visible" :key="i">
          <p v-if="s.title" class="mb-1 px-3 text-xs font-semibold tracking-wide text-slate-400 uppercase dark:text-slate-500">
            {{ s.title }}
          </p>
          <ul class="space-y-0.5">
            <li v-for="item in s.items" :key="item.to">
              <RouterLink
                :to="item.to"
                :aria-current="isActive(item) ? 'page' : undefined"
                :class="[
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  isActive(item)
                    ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300'
                    : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white',
                ]"
              >
                <AppIcon :name="item.icon" />
                {{ item.label }}
              </RouterLink>
            </li>
          </ul>
        </div>
      </nav>

      <div class="border-t border-slate-200 p-3 dark:border-slate-800">
        <RouterLink
          to="/profile"
          class="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-slate-100 dark:hover:bg-slate-800"
          :aria-current="route.path.startsWith('/profile') ? 'page' : undefined"
        >
          <span
            class="flex size-9 shrink-0 items-center justify-center rounded-full bg-brand-100 text-sm font-semibold text-brand-800 dark:bg-brand-900 dark:text-brand-200"
            aria-hidden="true"
          >
            {{ initials(displayName) }}
          </span>
          <span class="min-w-0">
            <span class="block truncate text-sm font-medium text-slate-900 dark:text-white">{{ displayName }}</span>
            <span class="block truncate text-xs text-slate-500 dark:text-slate-400">{{ roleText }}</span>
          </span>
        </RouterLink>
      </div>
    </aside>

    <div class="lg:pl-64">
      <header
        class="sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-slate-200 bg-white/80 px-4 backdrop-blur sm:px-6 dark:border-slate-800 dark:bg-slate-950/80"
      >
        <button
          type="button"
          class="btn btn-ghost px-2 lg:hidden"
          aria-label="Open menu"
          aria-controls="sidebar"
          :aria-expanded="mobileOpen"
          @click="mobileOpen = true"
        >
          <AppIcon name="menu" />
        </button>
        <div class="ml-auto flex items-center gap-2">
          <HealthBadge />
          <ThemeToggle />
          <button type="button" class="btn btn-ghost" @click="auth.logout('manual')">
            <AppIcon name="logout" />
            <span class="hidden sm:inline">Sign out</span>
          </button>
        </div>
      </header>

      <main id="main" class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <RouterView />
      </main>
    </div>
  </div>
</template>
