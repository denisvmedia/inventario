import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import PageSection from '../PageSection.vue'

describe('PageSection', () => {
  it('renders the default slot inside a <section>', () => {
    const wrapper = mount(PageSection, {
      slots: { default: () => h('p', 'body content') },
    })

    expect(wrapper.element.tagName).toBe('SECTION')
    expect(wrapper.text()).toContain('body content')
  })

  it('omits the heading row entirely when no title is provided', () => {
    const wrapper = mount(PageSection)

    expect(wrapper.find('h2').exists()).toBe(false)
    expect(wrapper.find('h3').exists()).toBe(false)
  })

  it('renders the title as <h2> by default with the matching size class', () => {
    const wrapper = mount(PageSection, { props: { title: 'General' } })

    const heading = wrapper.get('h2')
    expect(heading.text()).toBe('General')
    expect(heading.classes()).toContain('text-lg')
  })

  it('honours the configured heading level', () => {
    const wrapper = mount(PageSection, { props: { title: 'Sub', as: 'h3' } })

    expect(wrapper.find('h2').exists()).toBe(false)
    expect(wrapper.get('h3').text()).toBe('Sub')
  })

  it('renders the description prop and the actions slot', () => {
    const wrapper = mount(PageSection, {
      props: { title: 'Permissions', description: 'Who can see this group' },
      slots: { actions: () => h('button', { class: 'action-marker' }, 'Invite') },
    })

    expect(wrapper.text()).toContain('Who can see this group')
    expect(wrapper.find('.action-marker').exists()).toBe(true)
  })

  it('description slot overrides the description prop', () => {
    const wrapper = mount(PageSection, {
      props: { title: 't', description: 'fallback' },
      slots: { description: () => h('span', { class: 'd-marker' }, 'from-slot') },
    })

    expect(wrapper.find('.d-marker').exists()).toBe(true)
    expect(wrapper.text()).toContain('from-slot')
    expect(wrapper.text()).not.toContain('fallback')
  })

  it('forwards data-testid and merges class', () => {
    const wrapper = mount(PageSection, {
      props: { title: 'x', testId: 'sect', class: 'extra-marker' },
    })

    expect(wrapper.attributes('data-testid')).toBe('sect')
    expect(wrapper.classes()).toContain('extra-marker')
  })
})
