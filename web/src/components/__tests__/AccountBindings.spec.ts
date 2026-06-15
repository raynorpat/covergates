import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import AccountBindings from '@/components/AccountBindings.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components })

describe('AccountBindings', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('shows linked check vs Link button', async () => {
    ;(http.get as any).mockResolvedValue({ data: { github: true, gitea: false } })
    const w = mount(AccountBindings, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('GitHub')
    expect(w.find('[aria-label="linked"]').exists()).toBe(true)
    expect(w.text()).toContain('Link')
  })
})
