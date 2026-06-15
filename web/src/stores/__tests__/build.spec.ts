import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useBuildStore } from '@/stores/build'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const repoPath = '/api/v1/repos/github/o/r'

describe('build store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('fetchList populates list', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ number: 1 }, { number: 2 }] })
    const s = useBuildStore()
    await s.fetchList(repoPath)
    expect(s.list).toHaveLength(2)
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/builds`)
  })

  it('fetchBuild loads current and the base build when baseBuildNumber>0', async () => {
    ;(http.get as any)
      .mockResolvedValueOnce({ data: { number: 5, baseBuildNumber: 3, sourceFiles: [] } })
      .mockResolvedValueOnce({ data: { number: 3, baseBuildNumber: 0, sourceFiles: [] } })
    const s = useBuildStore()
    await s.fetchBuild(repoPath, 5)
    expect(s.current?.number).toBe(5)
    expect(s.base?.number).toBe(3)
    expect(http.get).toHaveBeenNthCalledWith(1, `${repoPath}/builds/5`)
    expect(http.get).toHaveBeenNthCalledWith(2, `${repoPath}/builds/3`)
  })

  it('fetchBuild leaves base null when baseBuildNumber is 0', async () => {
    ;(http.get as any).mockResolvedValue({ data: { number: 5, baseBuildNumber: 0 } })
    const s = useBuildStore()
    await s.fetchBuild(repoPath, 5)
    expect(s.base).toBeNull()
    expect(http.get).toHaveBeenCalledTimes(1)
  })

  it('fetchSource returns text', async () => {
    ;(http.get as any).mockResolvedValue({ data: 'line1\nline2' })
    const s = useBuildStore()
    const text = await s.fetchSource(repoPath, 'a/b.go', 'sha1')
    expect(text).toBe('line1\nline2')
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/content/a/b.go`, { params: { gitref: 'sha1' } })
  })

  it('fetchChanges hits the pulls changes endpoint', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ path: 'a.go', added: true, renamed: false, deleted: false }] })
    const s = useBuildStore()
    const changes = await s.fetchChanges(repoPath, 3)
    expect(changes).toHaveLength(1)
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/pulls/3/changes`)
  })
})
