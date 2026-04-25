import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'

import MediaGallery from '../MediaGallery.vue'

describe('MediaGallery', () => {
  it('renders default-slot children', () => {
    const wrapper = mount(MediaGallery, {
      slots: { default: () => [h('div', { class: 'item' }, 'A'), h('div', { class: 'item' }, 'B')] },
    })

    expect(wrapper.findAll('.item').length).toBe(2)
  })

  it('defaults to density="default"', () => {
    const wrapper = mount(MediaGallery)

    expect(wrapper.attributes('data-density')).toBe('default')
    expect(wrapper.classes()).toContain('grid')
    expect(wrapper.classes()).toContain('lg:grid-cols-4')
  })

  it('applies the compact variant classes', () => {
    const wrapper = mount(MediaGallery, { props: { density: 'compact' } })

    expect(wrapper.attributes('data-density')).toBe('compact')
    expect(wrapper.classes()).toContain('grid')
    expect(wrapper.classes()).toContain('gap-3')
    expect(wrapper.classes()).toContain('xl:grid-cols-6')
  })

  it('applies the relaxed variant classes', () => {
    const wrapper = mount(MediaGallery, { props: { density: 'relaxed' } })

    expect(wrapper.attributes('data-density')).toBe('relaxed')
    expect(wrapper.classes()).toContain('lg:grid-cols-3')
  })

  it('forwards testId', () => {
    const wrapper = mount(MediaGallery, { props: { testId: 'gallery' } })

    expect(wrapper.attributes('data-testid')).toBe('gallery')
  })

  it('renders as a <ul> when as="ul"', () => {
    const wrapper = mount(MediaGallery, { props: { as: 'ul' } })

    expect(wrapper.element.tagName).toBe('UL')
  })

  it('renders as a <section> when as="section"', () => {
    const wrapper = mount(MediaGallery, { props: { as: 'section' } })

    expect(wrapper.element.tagName).toBe('SECTION')
  })

  it('merges a caller-supplied class with the variant classes', () => {
    const wrapper = mount(MediaGallery, { props: { class: 'extra-marker' } })

    expect(wrapper.classes()).toContain('extra-marker')
    expect(wrapper.classes()).toContain('grid')
  })
})
