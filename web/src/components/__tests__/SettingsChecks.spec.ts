import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsChecks from '@/components/SettingsChecks.vue'
import type { RepoSetting } from '@/types/setting'

const vuetify = createVuetify({ components, directives })

const setting: RepoSetting = {
  filters: [], mergePR: false, updateAction: '', protected: false,
  coverageMinimum: 80, coverageDecreaseThreshold: 2
}

describe('SettingsChecks', () => {
  it('emits save with edited check thresholds', async () => {
    const w = mount(SettingsChecks, {
      props: { modelValue: setting },
      global: { plugins: [vuetify] }
    })
    const inputs = w.findAll('input[type="number"]')
    await inputs[0].setValue('90')
    await inputs[1].setValue('5')
    await w.find('button').trigger('click')
    const events = w.emitted('save') as RepoSetting[][]
    expect(events).toBeTruthy()
    expect(events[0][0].coverageMinimum).toBe(90)
    expect(events[0][0].coverageDecreaseThreshold).toBe(5)
    expect(events[0][0].protected).toBe(false)
  })
})
