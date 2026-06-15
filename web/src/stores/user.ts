import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import http from '@/plugins/http'
import type { User } from '@/types'

export const useUserStore = defineStore('user', () => {
  const current = ref<User | null>(null)
  const isAuthenticated = computed(() => current.value !== null)

  async function fetch() {
    try {
      const { data } = await http.get<User>('/api/v1/user')
      current.value = data && data.login ? data : null
    } catch {
      current.value = null
    }
  }

  return { current, isAuthenticated, fetch }
})
