import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { Button } from '../index'

describe('Button', () => {
  it('renders a <button> by default with the default variant and size classes', () => {
    const wrapper = mount(Button, { slots: { default: 'Save' } })

    expect(wrapper.element.tagName).toBe('BUTTON')
    expect(wrapper.text()).toBe('Save')
    // default variant uses bg-primary; default size is h-9.
    expect(wrapper.classes()).toContain('bg-primary')
    expect(wrapper.classes()).toContain('h-9')
  })

  it('applies destructive variant classes', () => {
    const wrapper = mount(Button, {
      props: { variant: 'destructive' },
      slots: { default: 'Delete' },
    })

    expect(wrapper.classes()).toContain('bg-destructive')
    expect(wrapper.classes()).toContain('text-white')
  })

  it('merges a caller-supplied class with variant classes via cn()', () => {
    const wrapper = mount(Button, {
      props: { class: 'w-full extra-marker' },
      slots: { default: 'Full width' },
    })

    expect(wrapper.classes()).toContain('w-full')
    expect(wrapper.classes()).toContain('extra-marker')
  })

  it('renders as the configured element via the `as` prop', () => {
    const wrapper = mount(Button, {
      props: { as: 'a' },
      attrs: { href: '/commodities' },
      slots: { default: 'Link' },
    })

    expect(wrapper.element.tagName).toBe('A')
    expect(wrapper.attributes('href')).toBe('/commodities')
  })

  it('supports all size variants', () => {
    const sm = mount(Button, { props: { size: 'sm' }, slots: { default: 'S' } })
    const lg = mount(Button, { props: { size: 'lg' }, slots: { default: 'L' } })
    const icon = mount(Button, { props: { size: 'icon' }, slots: { default: 'I' } })

    expect(sm.classes()).toContain('h-8')
    expect(lg.classes()).toContain('h-10')
    expect(icon.classes()).toContain('size-9')
  })
})
