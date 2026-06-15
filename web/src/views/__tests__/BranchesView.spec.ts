import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import BranchesView from '@/views/BranchesView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components })
function makeRouter() {
  return createRouter({ history: createMemoryHistory(), routes: [{ path: '/report/:scm/:namespace/:name/branches', component: BranchesView }, { path: '/:p(.*)*', component: { template: '<div/>' } }] })
}

const mk = (number: number, branch: string, coverage: number) => ({
  number, branch, coverage, serviceName: '', serviceNumber: '', commit: 'c', pullRequest: 0, status: 'done',
  parallel: false, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '', authorName: '',
  authorEmail: '', createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
})

describe('BranchesView', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('shows the latest build per branch', async () => {
    ;(http.get as any).mockResolvedValue({ data: [mk(3, 'main', 0.8), mk(1, 'main', 0.5), mk(2, 'dev', 0.6)] })
    const router = makeRouter()
    router.push('/report/github/o/r/branches')
    await router.isReady()
    const w = mount(BranchesView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('main')
    expect(w.text()).toContain('dev')
    expect(w.text()).toContain('80.0%')
    expect(w.text()).not.toContain('50.0%')
  })
})
