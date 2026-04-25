import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import PageHeader from '../PageHeader.vue'

describe('PageHeader', () => {
  it('renders the title in an <h1> by default', () => {
    const wrapper = mount(PageHeader, { props: { title: 'Locations' } })

    const heading = wrapper.get('h1')
    expect(heading.text()).toBe('Locations')
    expect(heading.classes()).toContain('text-2xl')
  })

  it('uses the configured heading level', () => {
    const wrapper = mount(PageHeader, { props: { title: 'Sub', as: 'h2' } })

    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('h2').text()).toBe('Sub')
  })

  it('renders the description prop in a <p>', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Files', description: 'All uploads across this group' },
    })

    expect(wrapper.get('p').text()).toBe('All uploads across this group')
  })

  it('renders the description slot in place of the prop when both are present', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Files', description: 'fallback' },
      slots: { description: () => h('span', { class: 'desc-marker' }, 'slot description') },
    })

    expect(wrapper.find('.desc-marker').exists()).toBe(true)
    expect(wrapper.text()).toContain('slot description')
    expect(wrapper.text()).not.toContain('fallback')
  })

  it('renders breadcrumbs and actions slots', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Areas' },
      slots: {
        breadcrumbs: () => h('nav', { class: 'crumb-marker' }, 'Home / Areas'),
        actions: () => h('button', { class: 'action-marker' }, 'Add'),
      },
    })

    expect(wrapper.find('.crumb-marker').exists()).toBe(true)
    expect(wrapper.find('.action-marker').exists()).toBe(true)
  })

  it('forwards data-testid to the outermost <header>', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'X', testId: 'page-header' },
    })

    expect(wrapper.element.tagName).toBe('HEADER')
    expect(wrapper.attributes('data-testid')).toBe('page-header')
  })

  it('merges caller-supplied classes via cn()', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'X', class: 'extra-marker' },
    })

    expect(wrapper.classes()).toContain('extra-marker')
  })
})
