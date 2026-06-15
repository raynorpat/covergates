import { createRouter, createWebHistory } from 'vue-router'
import { basePath } from '@/lib/base'
import { useUserStore } from '@/stores/user'
import Home from '@/views/Home.vue'
import Login from '@/views/Login.vue'
import Dashboard from '@/views/Dashboard.vue'
import User from '@/views/User.vue'
import BuildsView from '@/views/BuildsView.vue'
import BuildDetailView from '@/views/BuildDetailView.vue'
import SourceView from '@/views/SourceView.vue'
import SettingsView from '@/views/SettingsView.vue'

const router = createRouter({
  history: createWebHistory(basePath() + '/'),
  routes: [
    { path: '/', name: 'home', component: Home },
    { path: '/login', name: 'login', component: Login },
    { path: '/repos', name: 'dashboard', component: Dashboard, meta: { requiresAuth: true } },
    { path: '/user', name: 'user', component: User, meta: { requiresAuth: true } },
    { path: '/report/:scm/:namespace/:name', name: 'repo', component: BuildsView },
    { path: '/report/:scm/:namespace/:name/settings', name: 'settings', component: SettingsView, meta: { requiresAuth: true } },
    { path: '/report/:scm/:namespace/:name/builds/:number', name: 'build', component: BuildDetailView },
    { path: '/report/:scm/:namespace/:name/builds/:number/source/:path(.*)*', name: 'source', component: SourceView },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
})

router.beforeEach((to) => {
  if (!to.meta.requiresAuth) return true
  const user = useUserStore()
  if (user.isAuthenticated) return true
  return { name: 'login', query: { redirect: to.fullPath } }
})

export default router
