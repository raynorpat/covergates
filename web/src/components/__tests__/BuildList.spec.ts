import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import BuildList from '@/components/BuildList.vue'
import type { Build } from '@/types/build'

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const build = (over: Partial<Build>): Build => ({
  number: 1, serviceName: '', serviceNumber: '', commit: 'abcdef1234', branch: 'main', pullRequest: 0,
  status: 'done', parallel: false, coverage: 0.82, coverageChange: 0.01, baseBuildID: 0, baseBuildNumber: 0,
  commitMessage: 'msg', authorName: 'a', authorEmail: 'e', createdAt: '2020-01-01T00:00:00Z', finishedAt: '', ...over
})

function mountList(builds: Build[]) {
  return mount(BuildList, { props: { builds, repoRoute: '/report/github/o/r' }, global: { plugins: [vuetify, router] } })
}

describe('BuildList', () => {
  it('renders rows and PR label', () => {
    const w = mountList([build({ number: 7, pullRequest: 0, branch: 'main' }), build({ number: 8, pullRequest: 45, branch: 'main' })])
    expect(w.text()).toContain('#7')
    expect(w.text()).toContain('#8')
    expect(w.text()).toContain('PR #45 → main')
  })

  it('shows empty state', () => {
    const w = mountList([])
    expect(w.text()).toContain('No builds yet')
  })
})
