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
    const s = { filters: ['a'], mergePR: true, updateAction: 'merge', protected: false }
    ;(http.get as any).mockResolvedValue({ data: s })
    ;(http.post as any).mockResolvedValue({ data: s })
    const store = useRepositoryStore()
    await store.fetchSetting('/api/v1/repos/github/o/r')
    expect(store.setting).toEqual(s)
    await store.updateSetting('/api/v1/repos/github/o/r', s)
    expect(http.post).toHaveBeenCalledWith('/api/v1/repos/github/o/r/setting', s)
  })
})
