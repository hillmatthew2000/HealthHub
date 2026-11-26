import { createRouter, createWebHistory, type RouteLocationNormalized, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import type { Permission } from '@/lib/permissions'

declare module 'vue-router' {
  interface RouteMeta {
    /** Reachable without signing in. */
    public?: boolean
    /** Permission needed to open the page. */
    perm?: Permission
    title?: string
  }
}

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true, title: 'Sign in' } },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    children: [
      { path: '', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: 'Dashboard' } },
      {
        path: 'patients',
        name: 'patients',
        component: () => import('@/views/PatientsView.vue'),
        meta: { perm: ['patients', 'read'], title: 'Patients' },
      },
      {
        path: 'patients/new',
        name: 'patient-new',
        component: () => import('@/views/PatientFormView.vue'),
        meta: { perm: ['patients', 'create'], title: 'New patient' },
      },
      {
        path: 'patients/:id',
        name: 'patient',
        component: () => import('@/views/PatientDetailView.vue'),
        props: true,
        meta: { perm: ['patients', 'read'], title: 'Patient' },
      },
      {
        path: 'patients/:id/edit',
        name: 'patient-edit',
        component: () => import('@/views/PatientFormView.vue'),
        props: true,
        meta: { perm: ['patients', 'update'], title: 'Edit patient' },
      },
      {
        path: 'observations',
        name: 'observations',
        component: () => import('@/views/ObservationsView.vue'),
        meta: { perm: ['observations', 'read'], title: 'Observations' },
      },
      {
        path: 'observations/new',
        name: 'observation-new',
        component: () => import('@/views/ObservationFormView.vue'),
        meta: { perm: ['observations', 'create'], title: 'New observation' },
      },
      {
        path: 'observations/:id',
        name: 'observation',
        component: () => import('@/views/ObservationFormView.vue'),
        props: true,
        meta: { perm: ['observations', 'read'], title: 'Observation' },
      },
      {
        path: 'users',
        name: 'users',
        component: () => import('@/views/UsersView.vue'),
        meta: { perm: ['users', 'read'], title: 'Users' },
      },
      {
        path: 'users/new',
        name: 'user-new',
        component: () => import('@/views/UserCreateView.vue'),
        meta: { perm: ['users', 'create'], title: 'New user' },
      },
      {
        path: 'users/:id',
        name: 'user',
        component: () => import('@/views/UserEditView.vue'),
        props: true,
        meta: { perm: ['users', 'update'], title: 'Edit user' },
      },
      {
        path: 'audit',
        name: 'audit',
        component: () => import('@/views/AuditView.vue'),
        meta: { perm: ['audit-log', 'read'], title: 'Audit events' },
      },
      { path: 'profile', name: 'profile', component: () => import('@/views/ProfileView.vue'), meta: { title: 'Profile' } },
      {
        path: 'profile/password',
        name: 'change-password',
        component: () => import('@/views/ChangePasswordView.vue'),
        meta: { title: 'Change password' },
      },
      { path: 'forbidden', name: 'forbidden', component: () => import('@/views/ForbiddenView.vue'), meta: { title: 'No access' } },
      { path: ':rest(.*)*', name: 'not-found', component: () => import('@/views/NotFoundView.vue'), meta: { title: 'Not found' } },
    ],
  },
]

/** Decides where a navigation may go. Exported for tests. */
export function guard(to: RouteLocationNormalized) {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (to.name === 'login' && auth.isAuthenticated) return { name: 'dashboard' }
    return true
  }
  if (!auth.isAuthenticated) {
    if (auth.token) auth.logout('expired')
    return { name: 'login', query: to.fullPath !== '/' ? { redirect: to.fullPath } : {} }
  }
  if (to.meta.perm && !auth.can(...to.meta.perm)) return { name: 'forbidden' }
  return true
}

export const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: (_to, _from, saved) => saved ?? { top: 0 },
})

router.beforeEach(guard)

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · HealthHub` : 'HealthHub'
})

/** Only follow redirects to paths inside the app. */
export function safeRedirect(value: unknown): string {
  return typeof value === 'string' && /^\/(?![/\\])/.test(value) ? value : '/'
}
