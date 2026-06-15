import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useUserStore } from '@/stores/user'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

describe('user store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('fetch() populates current and isAuthenticated', async () => {
    ;(http.get as any).mockResolvedValue({ data: { login: 'alice' } })
    const store = useUserStore()
    expect(store.isAuthenticated).toBe(false)
    await store.fetch()
    expect(store.current?.login).toBe('alice')
    expect(store.isAuthenticated).toBe(true)
  })

  it('fetch() leaves user null on error', async () => {
    ;(http.get as any).mockRejectedValue(new Error('401'))
    const store = useUserStore()
    await store.fetch()
    expect(store.current).toBeNull()
    expect(store.isAuthenticated).toBe(false)
  })

  it('fetchScm populates provider map', async () => {
    ;(http.get as any).mockResolvedValue({ data: { github: true, gitea: false } })
    const store = useUserStore()
    await store.fetchScm()
    expect(store.scm.github).toBe(true)
    expect(store.scm.gitea).toBe(false)
  })
})
