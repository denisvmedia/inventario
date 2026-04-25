import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import AuthCard from '../AuthCard.vue'

describe('AuthCard', () => {
  it('renders the brand h1 "Inventario" required by the auth e2e suite', () => {
    const wrapper = mount(AuthCard, { slots: { default: 'body' } })

    expect(wrapper.get('h1').text()).toBe('Inventario')
  })

  it('renders the subtitle when provided', () => {
    const wrapper = mount(AuthCard, { props: { subtitle: 'Create an account' } })

    expect(wrapper.text()).toContain('Create an account')
  })

  it('omits the subtitle paragraph when not provided', () => {
    const wrapper = mount(AuthCard, { slots: { default: 'body' } })
    const paragraphs = wrapper.findAll('p')

    expect(paragraphs.length).toBe(0)
  })

  it('exposes a banner slot rendered above the default body', () => {
    const wrapper = mount(AuthCard, {
      slots: {
        banner: () => h('div', { class: 'banner-marker' }, 'Notice'),
        default: () => h('div', { class: 'body-marker' }, 'Body'),
      },
    })

    const html = wrapper.html()
    expect(html.indexOf('banner-marker')).toBeGreaterThan(-1)
    expect(html.indexOf('body-marker')).toBeGreaterThan(-1)
    expect(html.indexOf('banner-marker')).toBeLessThan(html.indexOf('body-marker'))
  })

  it('renders the footer slot only when content is provided', () => {
    const without = mount(AuthCard, { slots: { default: 'body' } })
    expect(without.find('.footer-marker').exists()).toBe(false)

    const wrapper = mount(AuthCard, {
      slots: {
        default: 'body',
        footer: () => h('a', { class: 'footer-marker' }, 'Back'),
      },
    })

    expect(wrapper.find('.footer-marker').exists()).toBe(true)
  })

  it('forwards testId and merges custom classes onto the wrapper', () => {
    const wrapper = mount(AuthCard, {
      props: { testId: 'auth-card', class: 'extra-marker' },
    })

    expect(wrapper.attributes('data-testid')).toBe('auth-card')
    expect(wrapper.classes()).toContain('extra-marker')
  })
})
