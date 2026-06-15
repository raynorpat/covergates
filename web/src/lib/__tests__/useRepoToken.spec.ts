import { describe, it, expect, vi, beforeEach } from 'vitest'
import http from '@/plugins/http'
import { useRepoToken } from '@/lib/useRepoToken'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const repoPath = '/api/v1/repos/github/o/r'

describe('useRepoToken', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reveal fetches the token', async () => {
    ;(http.get as any).mockResolvedValue({ data: { token: 'sec' } })
    const t = useRepoToken(repoPath)
    await t.reveal()
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/token`)
    expect(t.token.value).toBe('sec')
    expect(t.revealed.value).toBe(true)
  })

  it('rotate patches the token', async () => {
    ;(http.patch as any).mockResolvedValue({ data: { token: 'new' } })
    const t = useRepoToken(repoPath)
    await t.rotate()
    expect(http.patch).toHaveBeenCalledWith(`${repoPath}/token`)
    expect(t.token.value).toBe('new')
  })
})
