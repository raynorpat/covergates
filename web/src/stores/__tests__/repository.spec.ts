import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useRepositoryStore } from '@/stores/repository'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

describe('repository store', () => {
  beforeEach(() => setActivePinia(createPinia()))

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
})
