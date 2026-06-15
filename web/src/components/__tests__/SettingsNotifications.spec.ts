import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsNotifications from '@/components/SettingsNotifications.vue'
import type { RepoSetting } from '@/types/setting'

const vuetify = createVuetify({ components, directives })

const setting: RepoSetting = {
  filters: [], mergePR: false, updateAction: '', protected: false,
  coverageMinimum: 0, coverageDecreaseThreshold: 0, disablePRComment: false,
  emailRecipients: ['a@x.com'], slackWebhook: '', notifyTrigger: 'failure'
}

describe('SettingsNotifications', () => {
  it('emits save with edited recipients and webhook', async () => {
    const w = mount(SettingsNotifications, {
      props: { modelValue: setting },
      global: { plugins: [vuetify] }
    })
    const textarea = w.find('textarea')
    await textarea.setValue('a@x.com\nb@x.com\n')
    const inputs = w.findAll('input[type="text"]')
    const slackInput = inputs[inputs.length - 1]
    await slackInput.setValue('https://hooks.slack.com/abc')
    await w.find('button').trigger('click')
    const events = w.emitted('save') as RepoSetting[][]
    expect(events).toBeTruthy()
    expect(events[0][0].emailRecipients).toEqual(['a@x.com', 'b@x.com'])
    expect(events[0][0].slackWebhook).toBe('https://hooks.slack.com/abc')
    // unrelated fields preserved
    expect(events[0][0].notifyTrigger).toBe('failure')
  })
})
