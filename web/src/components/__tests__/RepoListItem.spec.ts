import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import http from '@/plugins/http'
import RepoListItem from '@/components/RepoListItem.vue'
import type { Repository } from '@/types'

vi.mock('@/plugins/http', () => ({
  default: { get: vi.fn(), post: vi.fn(), patch: vi.fn() },
  errorMessage: (e: unknown) => String(e)
}))

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }, { path: '/report/:scm/:namespace/:name', component: { template: '<div/>' } }] })

const baseRepo: Repository = { ID: 1, URL: 'u', ReportID: '', NameSpace: 'o', Name: 'r', Branch: 'main', Private: false, SCM: 'github' }

function mountItem(repo: Repository) {
  return mount(RepoListItem, { props: { repo }, global: { plugins: [vuetify, router] } })
}

describe('RepoListItem', () => {
  beforeEach(() => vi.clearAllMocks())

  it('activate: GET ok then PATCH report, emits activated', async () => {
    ;(http.get as any).mockResolvedValue({})
    ;(http.patch as any).mockResolvedValue({})
    const w = mountItem({ ...baseRepo })
    await w.find('button').trigger('click')
    await Promise.resolve(); await Promise.resolve()
    expect(http.patch).toHaveBeenCalledWith('/api/v1/repos/github/o/r/report')
    expect(w.emitted('activated')).toBeTruthy()
  })

  it('activate: GET 404 then POST create then PATCH report', async () => {
    ;(http.get as any).mockRejectedValue(new Error('404'))
    ;(http.post as any).mockResolvedValue({})
    ;(http.patch as any).mockResolvedValue({})
    const w = mountItem({ ...baseRepo })
    await w.find('button').trigger('click')
    await Promise.resolve(); await Promise.resolve(); await Promise.resolve()
    expect(http.post).toHaveBeenCalledWith('/api/v1/repos', expect.objectContaining({ Name: 'r' }))
    expect(http.patch).toHaveBeenCalledWith('/api/v1/repos/github/o/r/report')
  })

  it('activated repo reveals token via GET /token', async () => {
    ;(http.get as any).mockResolvedValue({ data: { token: 'secret123' } })
    const w = mountItem({ ...baseRepo, ReportID: 'abc' })
    const showBtn = w.findAll('button').find((b) => b.text().includes('Show token'))!
    await showBtn.trigger('click')
    await Promise.resolve(); await Promise.resolve()
    expect(http.get).toHaveBeenCalledWith('/api/v1/repos/github/o/r/token')
    expect(w.html()).toContain('secret123')
  })
})
