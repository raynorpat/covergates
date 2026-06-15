import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import http from '@/plugins/http'
import type { User } from '@/types'

export const useUserStore = defineStore('user', () => {
  const current = ref<User | null>(null)
  const scm = ref<Record<string, boolean>>({})
  const isAuthenticated = computed(() => current.value !== null)

  async function fetch() {
    try {
      const { data } = await http.get<User>('/api/v1/user')
      current.value = data && data.login ? data : null
    } catch {
      current.value = null
    }
  }

  async function fetchScm() {
    try {
      const { data } = await http.get<Record<string, boolean>>('/api/v1/user/scm')
      scm.value = data ?? {}
    } catch {
      scm.value = {}
    }
  }

  return { current, scm, isAuthenticated, fetch, fetchScm }
})
