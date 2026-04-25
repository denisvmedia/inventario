import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { X } from 'lucide-vue-next'
import IconButton from '../IconButton.vue'

describe('IconButton', () => {
  it('renders a <button type="button"> by default with the supplied aria-label', () => {
    const wrapper = mount(IconButton, {
      props: { ariaLabel: 'Close dialog' },
      slots: { default: () => h(X) },
    })

    expect(wrapper.element.tagName).toBe('BUTTON')
    expect(wrapper.attributes('type')).toBe('button')
    expect(wrapper.attributes('aria-label')).toBe('Close dialog')
  })

  it('defaults to ghost / icon variants from the shadcn Button', () => {
    const wrapper = mount(IconButton, {
      props: { ariaLabel: 'Close' },
      slots: { default: () => h(X) },
    })

    // ghost has no bg-primary; size=icon gives size-9 from the cva variant.
    expect(wrapper.classes()).toContain('size-9')
    expect(wrapper.classes()).not.toContain('bg-primary')
  })

  it('respects an overridden variant and size', () => {
    const wrapper = mount(IconButton, {
      props: { ariaLabel: 'Delete', variant: 'destructive', size: 'icon-sm' },
      slots: { default: () => h(X) },
    })

    expect(wrapper.classes()).toContain('bg-destructive')
    expect(wrapper.classes()).toContain('size-8')
  })

  it('forwards type, disabled, class and data-testid', () => {
    const wrapper = mount(IconButton, {
      props: {
        ariaLabel: 'Submit',
        type: 'submit',
        disabled: true,
        class: 'extra-marker',
        testId: 'submit-icon',
      },
      slots: { default: () => h(X) },
    })

    expect(wrapper.attributes('type')).toBe('submit')
    expect(wrapper.attributes('disabled')).toBeDefined()
    expect(wrapper.attributes('data-testid')).toBe('submit-icon')
    expect(wrapper.classes()).toContain('extra-marker')
  })

  it('emits click events with the underlying MouseEvent', async () => {
    const wrapper = mount(IconButton, {
      props: { ariaLabel: 'Close' },
      slots: { default: () => h(X) },
    })

    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
    expect(wrapper.emitted('click')![0][0]).toBeInstanceOf(MouseEvent)
  })

  it('renders the icon supplied via the default slot', () => {
    const wrapper = mount(IconButton, {
      props: { ariaLabel: 'Close' },
      slots: { default: () => h(X, { class: 'icon-marker' }) },
    })

    expect(wrapper.find('.icon-marker').exists()).toBe(true)
  })
})
