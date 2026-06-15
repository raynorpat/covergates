import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import EmbedCard from '@/components/EmbedCard.vue'

const vuetify = createVuetify({ components })

describe('EmbedCard', () => {
  it('builds badge markdown from reportId', () => {
    const w = mount(EmbedCard, {
      props: { reportId: 'rid123', repoRoute: '/report/github/o/r' },
      global: { plugins: [vuetify] }
    })
    expect(w.html()).toContain('/api/v1/reports/rid123/badge')
    const inputs = w.findAll('input')
    const badge = inputs.find((i) => (i.element as HTMLInputElement).value.includes('/badge'))
    expect(badge).toBeTruthy()
    expect((badge!.element as HTMLInputElement).value).toContain('![Coverage]')
  })
})
