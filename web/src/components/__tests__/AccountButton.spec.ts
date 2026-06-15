import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import AccountButton from '@/components/AccountButton.vue'
import { useUserStore } from '@/stores/user'

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }] })

function mountButton() {
  return mount(AccountButton, { global: { plugins: [vuetify, router] } })
}

describe('AccountButton', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows Login when unauthenticated', () => {
    const w = mountButton()
    expect(w.text()).toContain('Login')
  })

  it('shows account menu when authenticated', () => {
    const store = useUserStore()
    store.current = { login: 'alice' }
    const w = mountButton()
    expect(w.text()).not.toContain('Login')
    expect(w.find('[aria-label="account"]').exists()).toBe(true)
  })
})
