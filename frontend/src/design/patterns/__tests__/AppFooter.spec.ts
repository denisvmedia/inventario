import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import AppFooter from '../AppFooter.vue'

describe('AppFooter', () => {
  it('renders inside a <footer> with the app-footer testid', () => {
    const wrapper = mount(AppFooter)

    expect(wrapper.element.tagName).toBe('FOOTER')
    expect(wrapper.attributes('data-testid')).toBe('app-footer')
  })

  it('shows the current year next to the copyright string', () => {
    const wrapper = mount(AppFooter)

    expect(wrapper.text()).toContain('Inventario')
    expect(wrapper.text()).toContain(String(new Date().getFullYear()))
  })
})
