import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import Home from '@/views/Home.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [
  { path: '/', component: { template: '<div/>' } },
  { path: '/repos', component: { template: '<div/>' } },
  { path: '/report/:scm/:namespace/:name', component: { template: '<div/>' } }
] })

function routeGet(map: Record<string, unknown>) {
  ;(http.get as any).mockImplementation((url: string) => {
    for (const key of Object.keys(map)) {
      if (url.endsWith(key)) return Promise.resolve({ data: map[key] })
    }
    return Promise.resolve({ data: null })
  })
}

describe('Home', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('shows stats and top repos when authenticated', async () => {
    routeGet({
      '/api/v1/user': { login: 'me', email: 'me@x.com' },
      '/api/v1/user/repos': [{ ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'a', Branch: 'm', Private: false, SCM: 'github' }],
      '/api/v1/user/stats': { repoCount: 3, activatedCount: 2, averageCoverage: 0.842, topRepos: [
        { scm: 'github', namespace: 'o', name: 'a', reportID: 'x', coverage: 0.9 }
      ] }
    })
    const w = mount(Home, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('o/a')      // top repo
    expect(w.text()).toContain('84.2%')    // average coverage tile
    expect(w.text()).not.toContain('Get started')
  })

  it('degrades gracefully when stats fail', async () => {
    ;(http.get as any).mockImplementation((url: string) => {
      if (url.endsWith('/api/v1/user')) return Promise.resolve({ data: { login: 'me' } })
      if (url.endsWith('/api/v1/user/repos')) return Promise.resolve({ data: [{ ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'a', Branch: 'm', Private: false, SCM: 'github' }] })
      return Promise.reject(new Error('boom')) // /stats fails
    })
    const w = mount(Home, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Welcome')
  })

  it('shows the get-started hero when not authenticated', async () => {
    routeGet({ '/api/v1/user': null })
    const w = mount(Home, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Get started')
  })
})
