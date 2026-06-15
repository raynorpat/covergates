import { createRouter, createWebHistory } from 'vue-router'
import { basePath } from '@/lib/base'

export default createRouter({
  history: createWebHistory(basePath() + '/'),
  routes: [{ path: '/', name: 'home', component: () => import('@/App.vue') }]
})
