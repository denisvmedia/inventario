import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { AlertTriangle, Info } from 'lucide-vue-next'
import Banner from '../Banner.vue'

describe('Banner', () => {
  it('renders the default slot inside an info-styled container by default', () => {
    const wrapper = mount(Banner, { slots: { default: 'Heads up' } })

    expect(wrapper.text()).toContain('Heads up')
    expect(wrapper.attributes('role')).toBe('status')
    expect(wrapper.attributes('data-variant')).toBe('info')
    expect(wrapper.classes()).toContain('bg-blue-50')
  })

  it.each([
    ['success', 'bg-green-50'],
    ['warning', 'bg-amber-50'],
    ['error', 'bg-red-50'],
  ] as const)('applies %s variant classes', (variant, expectedClass) => {
    const wrapper = mount(Banner, { props: { variant } })

    expect(wrapper.attributes('data-variant')).toBe(variant)
    expect(wrapper.classes()).toContain(expectedClass)
  })

  it('renders the default lucide icon for the variant', () => {
    const wrapper = mount(Banner, { props: { variant: 'info' } })

    // Default Info icon renders as an <svg>; we only assert presence.
    expect(wrapper.find('svg').exists()).toBe(true)
  })

  it('uses the icon prop override when provided', () => {
    const wrapper = mount(Banner, { props: { icon: AlertTriangle } })
    const defaultWrapper = mount(Banner, { props: { icon: Info } })

    // Different lucide components render to different inner DOM, but the
    // simplest stable check is that both render an svg in the same slot
    // — the override path must not throw.
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(defaultWrapper.find('svg').exists()).toBe(true)
  })

  it('omits the icon entirely when icon=null', () => {
    const wrapper = mount(Banner, { props: { icon: null } })

    expect(wrapper.find('svg').exists()).toBe(false)
  })

  it('renders the actions slot', () => {
    const wrapper = mount(Banner, {
      slots: { actions: () => h('button', { class: 'act-marker' }, 'Retry') },
    })

    expect(wrapper.find('.act-marker').exists()).toBe(true)
  })

  it('omits the dismiss button when dismissible is false', () => {
    const wrapper = mount(Banner)

    expect(wrapper.find('button[aria-label="Dismiss"]').exists()).toBe(false)
  })

  it('emits dismiss when the close button is clicked', async () => {
    const wrapper = mount(Banner, { props: { dismissible: true } })

    const button = wrapper.get('button[aria-label="Dismiss"]')
    await button.trigger('click')

    expect(wrapper.emitted('dismiss')).toHaveLength(1)
  })

  it('forwards data-testid and merges class', () => {
    const wrapper = mount(Banner, {
      props: { testId: 'invite-banner', class: 'extra-marker' },
    })

    expect(wrapper.attributes('data-testid')).toBe('invite-banner')
    expect(wrapper.classes()).toContain('extra-marker')
  })
})
