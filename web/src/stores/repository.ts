import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Repository } from '@/types'
import type { RepoSetting } from '@/types/setting'

export const useRepositoryStore = defineStore('repository', () => {
  const list = ref<Repository[]>([])
  const setting = ref<RepoSetting | null>(null)

  async function fetchList() {
    const { data } = await http.get<Repository[]>('/api/v1/user/repos')
    list.value = data ?? []
  }

  async function synchronize() {
    await http.patch('/api/v1/user/repos')
    await fetchList()
  }

  async function fetchSetting(repoPath: string) {
    const { data } = await http.get<RepoSetting>(`${repoPath}/setting`)
    setting.value = data
  }

  async function updateSetting(repoPath: string, value: RepoSetting) {
    const { data } = await http.post<RepoSetting>(`${repoPath}/setting`, value)
    setting.value = data
  }

  return { list, setting, fetchList, synchronize, fetchSetting, updateSetting }
})
