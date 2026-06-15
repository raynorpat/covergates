import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsGeneral from '@/components/SettingsGeneral.vue'

const vuetify = createVuetify({ components, directives })

describe('SettingsGeneral', () => {
  it('emits save with edited filters', async () => {
    const w = mount(SettingsGeneral, {
      props: { modelValue: { filters: ['x'], mergePR: false, updateAction: 'merge', protected: false, coverageMinimum: 0, coverageDecreaseThreshold: 0, disablePRComment: false } },
      global: { plugins: [vuetify] }
    })
    const textarea = w.find('textarea')
    await textarea.setValue('a\nb\n')
    const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
    await saveBtn.trigger('click')
    const ev = w.emitted('save')
    expect(ev).toBeTruthy()
    expect((ev![0][0] as any).filters).toEqual(['a', 'b'])
  })
})
