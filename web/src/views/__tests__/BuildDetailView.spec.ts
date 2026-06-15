import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import BuildDetailView from '@/views/BuildDetailView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components, directives })

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/report/:scm/:namespace/:name/builds/:number', component: BuildDetailView }, { path: '/:p(.*)*', component: { template: '<div/>' } }]
  })
}

describe('BuildDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('loads and renders the build summary', async () => {
    ;(http.get as any).mockResolvedValue({ data: {
      number: 5, branch: 'main', commit: 'abcdef1234', commitMessage: 'hello', coverage: 0.82,
      coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, authorName: 'a', status: 'done',
      sourceFiles: [{ name: 'a.go', source_digest: '', coverage: [1, 0] }], jobs: [], createdAt: '2020-01-01T00:00:00Z'
    } })
    const router = makeRouter()
    router.push('/report/github/o/r/builds/5')
    await router.isReady()
    const w = mount(BuildDetailView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Build #5')
    expect(w.text()).toContain('a.go')
  })

  it('shows sign-in prompt on 401', async () => {
    ;(http.get as any).mockRejectedValue({ isAxiosError: true, response: { status: 401 } })
    const router = makeRouter()
    router.push('/report/github/o/r/builds/5')
    await router.isReady()
    const w = mount(BuildDetailView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Sign in to view this repository')
  })

  it('shows not-found on 404', async () => {
    ;(http.get as any).mockRejectedValue({ isAxiosError: true, response: { status: 404 } })
    const router = makeRouter()
    router.push('/report/github/o/r/builds/9')
    await router.isReady()
    const w = mount(BuildDetailView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Build not found')
  })
})
