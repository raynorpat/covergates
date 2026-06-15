import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import SettingsChecks from '@/components/SettingsChecks.vue'
import type { RepoSetting } from '@/types/setting'

const vuetify = createVuetify({ components })

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
    const comp: any = w.vm
    comp.minimum = 90
    comp.decrease = 5
    comp.save()
    const events = w.emitted('save') as RepoSetting[][]
    expect(events).toBeTruthy()
    expect(events[0][0].coverageMinimum).toBe(90)
    expect(events[0][0].coverageDecreaseThreshold).toBe(5)
    // unrelated fields preserved
    expect(events[0][0].protected).toBe(false)
  })
})
