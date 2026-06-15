import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import FileCoverageTree from '@/components/FileCoverageTree.vue'
import type { Build } from '@/types/build'

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const build = (files: any[]): Build => ({
  number: 5, serviceName: '', serviceNumber: '', commit: 'c', branch: 'main', pullRequest: 0, status: 'done',
  parallel: false, coverage: 0.5, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '',
  authorName: '', authorEmail: '', sourceFiles: files, jobs: [], createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
})

describe('FileCoverageTree', () => {
  it('renders folders and files', () => {
    const w = mount(FileCoverageTree, {
      props: {
        build: build([
          { name: 'a/b.go', source_digest: '', coverage: [1, 0] },
          { name: 'd.go', source_digest: '', coverage: [1] }
        ]),
        buildRoute: '/report/github/o/r/builds/5'
      },
      global: { plugins: [vuetify, router] }
    })
    expect(w.text()).toContain('a')
    expect(w.text()).toContain('b.go')
    expect(w.text()).toContain('d.go')
  })
})
