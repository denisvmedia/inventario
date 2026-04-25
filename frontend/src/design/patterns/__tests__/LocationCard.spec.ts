import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import LocationCard from '../LocationCard.vue'

describe('LocationCard', () => {
  it('renders the name in an <h3>', () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    expect(wrapper.get('h3').text()).toBe('Office')
  })

  it('renders the address when provided', () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', address: '123 Main' },
    })

    expect(wrapper.text()).toContain('123 Main')
  })

  it('omits the address paragraph when not provided', () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    expect(wrapper.findAll('p').some((p) => p.text() === '123 Main')).toBe(false)
  })

  it('renders the value label when provided', () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', valueLabel: '100.00 USD' },
    })

    expect(wrapper.text()).toContain('100.00 USD')
  })

  it('shows "Loading…" instead of the value when loadingValue is true', () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', valueLabel: '100.00 USD', loadingValue: true },
    })

    expect(wrapper.text()).toContain('Loading…')
    expect(wrapper.text()).not.toContain('100.00 USD')
  })

  it('keeps the .location-card legacy anchor on the outermost element', () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    expect(wrapper.classes()).toContain('location-card')
  })

  it('forwards testId to the outermost element', () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', testId: 'loc-1' },
    })

    expect(wrapper.attributes('data-testid')).toBe('loc-1')
  })

  it('exposes role="button" + aria-expanded when expandable', () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', expanded: true },
    })

    expect(wrapper.attributes('role')).toBe('button')
    expect(wrapper.attributes('aria-expanded')).toBe('true')
    expect(wrapper.attributes('tabindex')).toBe('0')
  })

  it('falls back to role="article" when not expandable', () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', expandable: false },
    })

    expect(wrapper.attributes('role')).toBe('article')
    expect(wrapper.attributes('aria-expanded')).toBeUndefined()
    expect(wrapper.attributes('tabindex')).toBeUndefined()
  })

  it('emits toggle on click when expandable', async () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    await wrapper.trigger('click')

    expect(wrapper.emitted('toggle')).toHaveLength(1)
  })

  it('emits toggle on Enter and Space when expandable', async () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    await wrapper.trigger('keydown', { key: 'Enter' })
    await wrapper.trigger('keydown', { key: ' ' })

    expect(wrapper.emitted('toggle')).toHaveLength(2)
  })

  it('does not emit toggle when expandable is false', async () => {
    const wrapper = mount(LocationCard, {
      props: { name: 'Office', expandable: false },
    })

    await wrapper.trigger('click')
    await wrapper.trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('toggle')).toBeUndefined()
  })

  it('exposes legacy title attributes on the action buttons', () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    expect(wrapper.get('[aria-label="View location"]').attributes('title')).toBe('View')
    expect(wrapper.get('[aria-label="Edit location"]').attributes('title')).toBe('Edit')
    expect(wrapper.get('[aria-label="Delete location"]').attributes('title')).toBe('Delete')
  })

  it('emits view, edit, delete without bubbling toggle from the action buttons', async () => {
    const wrapper = mount(LocationCard, { props: { name: 'Office' } })

    await wrapper.get('[aria-label="View location"]').trigger('click')
    await wrapper.get('[aria-label="Edit location"]').trigger('click')
    await wrapper.get('[aria-label="Delete location"]').trigger('click')

    expect(wrapper.emitted('view')).toHaveLength(1)
    expect(wrapper.emitted('edit')).toHaveLength(1)
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('toggle')).toBeUndefined()
  })
})
