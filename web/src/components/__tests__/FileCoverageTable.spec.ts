import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import FileCoverageTable from '@/components/FileCoverageTable.vue'
import type { Build } from '@/types/build'

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const p = (v: number) => v
const build = (files: any[]): Build => ({
  number: 5, serviceName: '', serviceNumber: '', commit: 'c', branch: 'main', pullRequest: 0, status: 'done',
  parallel: false, coverage: 0.5, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '',
  authorName: '', authorEmail: '', sourceFiles: files, jobs: [], createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
})

describe('FileCoverageTable', () => {
  it('computes per-file coverage and delta vs base', () => {
    const current = build([{ name: 'a.go', source_digest: '', coverage: [p(1), p(1), p(0), p(0)] }]) // 50%
    const base = build([{ name: 'a.go', source_digest: '', coverage: [p(1), p(0), p(0), p(0)] }]) // 25%
    const w = mount(FileCoverageTable, {
      props: { build: current, base, buildRoute: '/report/github/o/r/builds/5' },
      global: { plugins: [vuetify, router] }
    })
    expect(w.text()).toContain('a.go')
    expect(w.text()).toContain('50.0%')
    expect(w.text()).toContain('+25.0%') // delta 50% - 25%
  })

  it('shows dash delta when no base', () => {
    const current = build([{ name: 'a.go', source_digest: '', coverage: [p(1), p(0)] }])
    const w = mount(FileCoverageTable, {
      props: { build: current, base: null, buildRoute: '/r' },
      global: { plugins: [vuetify, router] }
    })
    expect(w.text()).toContain('a.go')
    expect(w.text()).toContain('—')
  })
})
