import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'

import StatCard from '../StatCard.vue'

describe('StatCard', () => {
  it('renders the label and value', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'Total Value', value: '1 000.00 USD' },
    })

    expect(wrapper.text()).toContain('Total Value')
    expect(wrapper.text()).toContain('1 000.00 USD')
  })

  it('falls back to em dash when value is missing', () => {
    const wrapper = mount(StatCard, { props: { label: 'Total Value' } })

    expect(wrapper.text()).toContain('—')
  })

  it('renders the description when provided', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'Files', value: '42', description: '12 MB used' },
    })

    expect(wrapper.text()).toContain('12 MB used')
  })

  it('renders the icon when provided', () => {
    const StarIcon = { template: '<svg class="star-marker" />' }
    const wrapper = mount(StatCard, {
      props: { label: 'X', value: '1', icon: StarIcon as never },
    })

    expect(wrapper.find('.star-marker').exists()).toBe(true)
  })

  it('shows a placeholder bar when loading', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'Loading', value: 'should not render', loading: true },
    })

    expect(wrapper.text()).not.toContain('should not render')
    expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="Loading"]').exists()).toBe(true)
  })

  it('applies the matching variant accent border', () => {
    const cases = [
      ['primary', 'border-l-primary'],
      ['success', 'border-l-success'],
      ['warning', 'border-l-warning'],
      ['destructive', 'border-l-destructive'],
    ] as const

    for (const [variant, expected] of cases) {
      const w = mount(StatCard, { props: { label: 'X', value: '1', variant } })
      expect(w.classes()).toContain(expected)
    }
  })

  it('does not render an accent border for the default variant', () => {
    const wrapper = mount(StatCard, { props: { label: 'X', value: '1' } })

    expect(wrapper.classes()).not.toContain('border-l-primary')
    expect(wrapper.classes()).not.toContain('border-l-4')
  })

  it('renders the value slot in place of the prop', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'X', value: 'fallback' },
      slots: { value: () => h('span', { class: 'custom-marker' }, 'CUSTOM') },
    })

    expect(wrapper.text()).toContain('CUSTOM')
    expect(wrapper.text()).not.toContain('fallback')
  })

  it('renders the actions slot when provided', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'X', value: '1' },
      slots: { actions: () => h('button', { class: 'act-marker' }, 'Go') },
    })

    expect(wrapper.find('.act-marker').exists()).toBe(true)
  })

  it('forwards testId', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'X', value: '1', testId: 'stat-1' },
    })

    expect(wrapper.attributes('data-testid')).toBe('stat-1')
  })
})
