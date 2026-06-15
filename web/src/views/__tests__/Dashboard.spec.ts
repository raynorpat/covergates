import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import Dashboard from '@/views/Dashboard.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }, { path: '/report/:scm/:namespace/:name', component: { template: '<div/>' } }] })

describe('Dashboard', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('lists repos and filters by search', async () => {
    ;(http.get as any).mockResolvedValue({ data: [
      { ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'alpha', Branch: 'm', Private: false, SCM: 'github' },
      { ID: 2, URL: 'u', ReportID: '', NameSpace: 'o', Name: 'beta', Branch: 'm', Private: false, SCM: 'github' }
    ] })
    const w = mount(Dashboard, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('o/alpha')
    expect(w.text()).toContain('o/beta')

    const input = w.find('input')
    await input.setValue('alpha')
    expect(w.text()).toContain('o/alpha')
    expect(w.text()).not.toContain('o/beta')
  })
})
