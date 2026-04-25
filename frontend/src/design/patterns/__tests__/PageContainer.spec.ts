import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PageContainer from '../PageContainer.vue'

describe('PageContainer', () => {
  it('renders a <main> by default with default-width and padded classes', () => {
    const wrapper = mount(PageContainer, { slots: { default: 'body' } })

    expect(wrapper.element.tagName).toBe('MAIN')
    expect(wrapper.text()).toBe('body')
    expect(wrapper.classes()).toContain('max-w-screen-xl')
    expect(wrapper.classes()).toContain('mx-auto')
    expect(wrapper.classes()).toContain('py-6')
  })

  it('applies the narrow width variant', () => {
    const wrapper = mount(PageContainer, { props: { width: 'narrow' } })

    expect(wrapper.classes()).toContain('max-w-2xl')
    expect(wrapper.classes()).not.toContain('max-w-screen-xl')
  })

  it('drops max-width and horizontal padding for the full variant', () => {
    const wrapper = mount(PageContainer, { props: { width: 'full' } })

    expect(wrapper.classes()).toContain('max-w-none')
    // tailwind-merge collapses the conflicting px-4/sm:px-6 with px-0/sm:px-0
    expect(wrapper.classes()).not.toContain('px-4')
  })

  it('omits vertical padding when padded=false', () => {
    const wrapper = mount(PageContainer, { props: { padded: false } })

    expect(wrapper.classes()).not.toContain('py-6')
    expect(wrapper.classes()).not.toContain('py-8')
  })

  it('respects the as prop and forwards data-testid', () => {
    const wrapper = mount(PageContainer, {
      props: { as: 'article', testId: 'profile-page' },
    })

    expect(wrapper.element.tagName).toBe('ARTICLE')
    expect(wrapper.attributes('data-testid')).toBe('profile-page')
  })

  it('merges caller-supplied classes via cn()', () => {
    const wrapper = mount(PageContainer, {
      props: { class: 'extra-marker bg-muted' },
    })

    expect(wrapper.classes()).toContain('extra-marker')
    expect(wrapper.classes()).toContain('bg-muted')
  })
})
