import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useRepositoryStore } from '@/stores/repository'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn(), post: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

describe('repository store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('fetchList() populates list', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ ID: 1, Name: 'r', NameSpace: 'o' }] })
    const store = useRepositoryStore()
    await store.fetchList()
    expect(store.list).toHaveLength(1)
    expect(store.list[0].Name).toBe('r')
  })

  it('synchronize() PATCHes then refetches', async () => {
    ;(http.patch as any).mockResolvedValue({})
    ;(http.get as any).mockResolvedValue({ data: [] })
    const store = useRepositoryStore()
    await store.synchronize()
    expect(http.patch).toHaveBeenCalledWith('/api/v1/user/repos')
    expect(http.get).toHaveBeenCalledWith('/api/v1/user/repos')
  })

  it('fetchSetting and updateSetting hit the setting endpoint', async () => {
    const s = { filters: ['a'], mergePR: true, updateAction: 'merge', protected: false, coverageMinimum: 0, coverageDecreaseThreshold: 0, disablePRComment: false, emailRecipients: [], slackWebhook: '', notifyTrigger: 'failure' }
    ;(http.get as any).mockResolvedValue({ data: s })
    ;(http.post as any).mockResolvedValue({ data: s })
    const store = useRepositoryStore()
    await store.fetchSetting('/api/v1/repos/github/o/r')
    expect(store.setting).toEqual(s)
    await store.updateSetting('/api/v1/repos/github/o/r', s)
    expect(http.post).toHaveBeenCalledWith('/api/v1/repos/github/o/r/setting', s)
  })

  it('ensureSynced syncs when the list is empty, once per session', async () => {
    ;(http.get as any).mockResolvedValue({ data: [] }) // empty list
    ;(http.patch as any).mockResolvedValue({ data: 'ok' })
    const store = useRepositoryStore()
    await store.ensureSynced()
    expect(http.patch).toHaveBeenCalledWith('/api/v1/user/repos') // synced because empty
    ;(http.patch as any).mockClear()
    await store.ensureSynced() // second call is a no-op (once guard)
    expect(http.patch).not.toHaveBeenCalled()
  })

  it('ensureSynced does not sync when repos already exist', async () => {
    ;(http.get as any).mockResolvedValue({ data: [
      { ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'a', Branch: 'm', Private: false, SCM: 'github' }
    ] })
    ;(http.patch as any).mockResolvedValue({ data: 'ok' })
    const store = useRepositoryStore()
    await store.ensureSynced()
    expect(http.patch).not.toHaveBeenCalled()
  })

  it('fetchStats loads the stats endpoint', async () => {
    const stats = { repoCount: 2, activatedCount: 1, averageCoverage: 0.8, topRepos: [] }
    ;(http.get as any).mockResolvedValue({ data: stats })
    const store = useRepositoryStore()
    await store.fetchStats()
    expect(http.get).toHaveBeenCalledWith('/api/v1/user/stats')
    expect(store.stats).toEqual(stats)
  })
})
