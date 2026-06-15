import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import ChangedFilesPanel from '@/components/ChangedFilesPanel.vue'
import type { Build } from '@/types/build'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const build: Build = {
  number: 5, serviceName: '', serviceNumber: '', commit: 'c', branch: 'feature', pullRequest: 7, status: 'done',
  parallel: false, coverage: 0.5, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '',
  authorName: '', authorEmail: '', sourceFiles: [{ name: 'a.go', source_digest: '', coverage: [1, 0] }], jobs: [],
  createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
}

function mountPanel() {
  return mount(ChangedFilesPanel, {
    props: { build, repoPath: '/api/v1/repos/github/o/r', buildRoute: '/report/github/o/r/builds/5', loginRedirect: '/x' },
    global: { plugins: [vuetify, router] }
  })
}

describe('ChangedFilesPanel', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('lists changed files with coverage', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ path: 'a.go', added: false, renamed: false, deleted: false }] })
    const w = mountPanel()
    await flushPromises()
    expect(w.text()).toContain('a.go')
    expect(w.text()).toContain('50.0%')
  })

  it('shows sign-in on 401', async () => {
    ;(http.get as any).mockRejectedValue({ isAxiosError: true, response: { status: 401 } })
    const w = mountPanel()
    await flushPromises()
    expect(w.text()).toContain('Sign in')
  })
})
