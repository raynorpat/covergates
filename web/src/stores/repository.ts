import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Repository, RepoStats } from '@/types'
import type { RepoSetting } from '@/types/setting'

export const useRepositoryStore = defineStore('repository', () => {
  const list = ref<Repository[]>([])
  const setting = ref<RepoSetting | null>(null)
  const stats = ref<RepoStats | null>(null)
  const autoSynced = ref(false)

  async function fetchList() {
    const { data } = await http.get<Repository[]>('/api/v1/user/repos')
    list.value = data ?? []
  }

  async function synchronize() {
    await http.patch('/api/v1/user/repos')
    await fetchList()
  }

  // ensureSynced syncs the user's repos from the SCM the first time they have
  // none, once per session. Returning users with repos are left untouched.
  async function ensureSynced() {
    if (autoSynced.value) return
    autoSynced.value = true
    try {
      await fetchList()
      if (list.value.length === 0) {
        await synchronize()
      }
    } catch (e) {
      autoSynced.value = false // release the guard so a later attempt can retry
      throw e
    }
  }

  async function fetchStats() {
    const { data } = await http.get<RepoStats>('/api/v1/user/stats')
    stats.value = data
  }

  async function fetchSetting(repoPath: string) {
    const { data } = await http.get<RepoSetting>(`${repoPath}/setting`)
    setting.value = data
  }

  async function updateSetting(repoPath: string, value: RepoSetting) {
    const { data } = await http.post<RepoSetting>(`${repoPath}/setting`, value)
    setting.value = data
  }

  return { list, setting, stats, fetchList, synchronize, ensureSynced, fetchStats, fetchSetting, updateSetting }
})
