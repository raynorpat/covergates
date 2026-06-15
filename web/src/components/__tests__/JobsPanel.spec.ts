import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import JobsPanel from '@/components/JobsPanel.vue'

const vuetify = createVuetify({ components })

describe('JobsPanel', () => {
  it('renders job flags and coverage', () => {
    const w = mount(JobsPanel, {
      props: { jobs: [{ id: 1, serviceJobID: 'j1', coverage: 0.8, flag: 'unit' }, { id: 2, serviceJobID: 'j2', coverage: 0.5, flag: '' }] },
      global: { plugins: [vuetify] }
    })
    expect(w.text()).toContain('unit')
    expect(w.text()).toContain('—')
    expect(w.text()).toContain('80.0%')
  })
})
