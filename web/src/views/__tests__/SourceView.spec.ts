import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import SourceView from '@/views/SourceView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
// SourceView calls axios.isAxiosError to detect the 401; stub it (vi.mock is hoisted).
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components })

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/report/:scm/:namespace/:name/builds/:number/source/:path(.*)*', component: SourceView },
      { path: '/:p(.*)*', component: { template: '<div/>' } }
    ]
  })
}

describe('SourceView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('shows sign-in prompt on 401 from content', async () => {
    ;(http.get as any).mockImplementation((url: string) => {
      if (url.includes('/builds/')) return Promise.resolve({ data: { number: 5, commit: 'c', baseBuildNumber: 0, sourceFiles: [{ name: 'a.go', source_digest: '', coverage: [1] }] } })
      return Promise.reject({ isAxiosError: true, response: { status: 401 } })
    })
    const router = makeRouter()
    router.push('/report/github/o/r/builds/5/source/a.go')
    await router.isReady()
    const w = mount(SourceView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Sign in to view source')
  })
})
