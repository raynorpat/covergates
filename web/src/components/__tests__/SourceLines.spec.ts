import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import SourceLines from '@/components/SourceLines.vue'

const vuetify = createVuetify({ components })

describe('SourceLines', () => {
  it('applies hit/miss classes and hit counts', () => {
    const w = mount(SourceLines, {
      props: { source: 'a\nb\nc', coverage: [3, 0, null], path: 'x.txt' },
      global: { plugins: [vuetify] }
    })
    const rows = w.findAll('tr')
    expect(rows).toHaveLength(3)
    expect(rows[0].classes()).toContain('statement-hit')
    expect(rows[0].text()).toContain('3×')
    expect(rows[1].classes()).toContain('statement-miss')
    expect(rows[2].classes()).not.toContain('statement-hit')
    expect(rows[2].classes()).not.toContain('statement-miss')
  })
})
