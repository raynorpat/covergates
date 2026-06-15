import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Repository } from '@/types'

export const useRepositoryStore = defineStore('repository', () => {
  const list = ref<Repository[]>([])

  async function fetchList() {
    const { data } = await http.get<Repository[]>('/api/v1/user/repos')
    list.value = data ?? []
  }

  async function synchronize() {
    await http.patch('/api/v1/user/repos')
    await fetchList()
  }

  return { list, fetchList, synchronize }
})
