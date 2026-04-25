import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import EmptyState from '../EmptyState.vue'

describe('EmptyState', () => {
  it('renders the title in an <h2>', () => {
    const wrapper = mount(EmptyState, { props: { title: 'No files yet' } })

    expect(wrapper.get('h2').text()).toBe('No files yet')
    expect(wrapper.attributes('role')).toBe('status')
  })

  it('renders the description prop', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 't', description: 'Upload one to get started' },
    })

    expect(wrapper.text()).toContain('Upload one to get started')
  })

  it('renders the description slot in place of the prop', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 't', description: 'fallback' },
      slots: { description: () => h('span', { class: 'd-marker' }, 'slot description') },
    })

    expect(wrapper.find('.d-marker').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('fallback')
  })

  it('renders an <img> when illustrationSrc is provided', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 't', illustrationSrc: '/x.svg', illustrationAlt: 'empty' },
    })

    const img = wrapper.get('img')
    expect(img.attributes('src')).toBe('/x.svg')
    expect(img.attributes('alt')).toBe('empty')
  })

  it('renders the illustration slot in place of the img', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 't', illustrationSrc: '/x.svg' },
      slots: { illustration: () => h('div', { class: 'custom-illust' }, 'X') },
    })

    expect(wrapper.find('.custom-illust').exists()).toBe(true)
    // The slot wins over the prop, so no <img> should render.
    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('does not render an actions container when no actions slot is provided', () => {
    const wrapper = mount(EmptyState, { props: { title: 't' } })

    expect(wrapper.findAll('div').some((d) => d.classes().includes('flex-wrap'))).toBe(false)
  })

  it('renders the actions slot when provided', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 't' },
      slots: { actions: () => h('button', { class: 'act-marker' }, 'Add') },
    })

    expect(wrapper.find('.act-marker').exists()).toBe(true)
  })

  it('forwards data-testid and merges class', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 't', testId: 'files-empty', class: 'extra-marker' },
    })

    expect(wrapper.attributes('data-testid')).toBe('files-empty')
    expect(wrapper.classes()).toContain('extra-marker')
  })
})
