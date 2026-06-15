import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Build } from '@/types/build'

export const useBuildStore = defineStore('build', () => {
  const list = ref<Build[]>([])
  const current = ref<Build | null>(null)
  const base = ref<Build | null>(null)

  async function fetchList(repoPath: string) {
    const { data } = await http.get<Build[]>(`${repoPath}/builds`)
    list.value = data ?? []
  }

  async function fetchBuild(repoPath: string, number: number) {
    const { data } = await http.get<Build>(`${repoPath}/builds/${number}`)
    current.value = data
    base.value = null
    if (data && data.baseBuildNumber > 0) {
      const res = await http.get<Build>(`${repoPath}/builds/${data.baseBuildNumber}`)
      base.value = res.data
    }
  }

  async function fetchSource(repoPath: string, path: string, commit: string): Promise<string> {
    const { data } = await http.get<string>(`${repoPath}/content/${path}`, { params: { gitref: commit } })
    return typeof data === 'string' ? data : String(data)
  }

  return { list, current, base, fetchList, fetchBuild, fetchSource }
})
