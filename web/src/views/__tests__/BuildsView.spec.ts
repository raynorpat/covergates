import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import BuildsView from '@/views/BuildsView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components, directives })

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/report/:scm/:namespace/:name', component: BuildsView }, { path: '/:p(.*)*', component: { template: '<div/>' } }]
  })
}

describe('BuildsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('renders builds on success', async () => {
    ;(http.get as any).mockResolvedValue({ data: [
      { number: 1, branch: 'main', commit: 'abcdef12', commitMessage: 'm', coverage: 0.8, coverageChange: 0, pullRequest: 0, status: 'done', createdAt: '2020-01-01T00:00:00Z' }
    ] })
    const router = makeRouter()
    router.push('/report/github/o/r')
    await router.isReady()
    const w = mount(BuildsView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('o/r')
    expect(w.text()).toContain('#1')
  })

  it('shows sign-in prompt on 401', async () => {
    ;(http.get as any).mockRejectedValue({ isAxiosError: true, response: { status: 401 } })
    const router = makeRouter()
    router.push('/report/github/o/r')
    await router.isReady()
    const w = mount(BuildsView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Sign in to view this repository')
  })
})
