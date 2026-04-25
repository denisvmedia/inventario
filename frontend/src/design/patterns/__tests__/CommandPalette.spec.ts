import { describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount } from '@vue/test-utils'

vi.mock('@/services/searchService', () => ({
  default: {
    search: vi.fn().mockResolvedValue({ data: [], meta: { total: 0 } }),
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import CommandPalette from '../CommandPalette.vue'

describe('CommandPalette', () => {
  it('mounts without errors and stays hidden when open=false', () => {
    setActivePinia(createPinia())

    const wrapper = mount(CommandPalette, {
      props: { open: false },
      global: {
        stubs: {
          Dialog: { template: '<div v-if="open"><slot /></div>', props: ['open'] },
          DialogContent: { template: '<div data-stub="dialog-content"><slot /></div>' },
          DialogHeader: { template: '<div><slot /></div>' },
          DialogTitle: { template: '<h2><slot /></h2>' },
          DialogDescription: { template: '<p><slot /></p>' },
        },
      },
    })

    // With open=false, the stubbed Dialog renders nothing.
    expect(wrapper.find('[data-stub="dialog-content"]').exists()).toBe(false)
  })

  it('renders the dialog content and the searchbox when open=true', () => {
    setActivePinia(createPinia())

    const wrapper = mount(CommandPalette, {
      props: { open: true },
      global: {
        stubs: {
          Dialog: { template: '<div v-if="open"><slot /></div>', props: ['open'] },
          DialogContent: { template: '<div data-stub="dialog-content"><slot /></div>' },
          DialogHeader: { template: '<div><slot /></div>' },
          DialogTitle: { template: '<h2><slot /></h2>' },
          DialogDescription: { template: '<p><slot /></p>' },
        },
      },
    })

    expect(wrapper.find('[data-stub="dialog-content"]').exists()).toBe(true)
    expect(wrapper.find('input[role="searchbox"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Type at least 2 characters to search.')
  })
})
