import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import {
  FormFooter,
  FormGrid,
  FormSection,
  formFooterVariants,
  formGridVariants,
  formSectionVariants,
} from '@design/patterns'

/**
 * PR 2.1 — layout patterns added by Phase 2 (#1327).
 *
 * The patterns are thin Tailwind/`cva` wrappers so the spec verifies
 * the contract callers depend on:
 *   - default class set, variant switches, caller `class` merging,
 *   - slot composition (title / description / default body),
 *   - accessibility wiring (aria-labelledby/-describedby on the
 *     section header), which is the only behaviour with non-trivial
 *     logic in this batch.
 */
describe('FormSection (PR 2.1)', () => {
  it('renders the default body without a header when no title/description provided', () => {
    const wrapper = mount(FormSection, {
      slots: { default: '<p>body</p>' },
    })

    expect(wrapper.find('header').exists()).toBe(false)
    expect(wrapper.find('[data-slot="form-section-body"]').html()).toContain('<p>body</p>')
  })

  it('renders title and description and links them via aria attributes', () => {
    const wrapper = mount(FormSection, {
      props: { title: 'Identity', description: 'Names and labels.' },
    })

    const section = wrapper.find('section')
    const heading = wrapper.find('h3')
    const description = wrapper.find('p')

    expect(heading.text()).toBe('Identity')
    expect(description.text()).toBe('Names and labels.')

    const labelledBy = section.attributes('aria-labelledby')
    const describedBy = section.attributes('aria-describedby')
    expect(labelledBy).toBeTruthy()
    expect(describedBy).toBeTruthy()
    expect(heading.attributes('id')).toBe(labelledBy)
    expect(description.attributes('id')).toBe(describedBy)
  })

  it('prefers slot content over the title/description props', () => {
    const wrapper = mount(FormSection, {
      props: { title: 'fallback' },
      slots: {
        title: '<span>Custom title</span>',
        description: '<em>Custom description</em>',
      },
    })

    expect(wrapper.find('h3').html()).toContain('<span>Custom title</span>')
    expect(wrapper.find('p').html()).toContain('<em>Custom description</em>')
  })

  it('applies the spacing variant and merges a caller class', () => {
    const wrapper = mount(FormSection, {
      props: { spacing: 'relaxed', class: 'extra-marker' },
    })

    expect(wrapper.classes()).toContain('gap-6')
    expect(wrapper.classes()).toContain('extra-marker')
  })

  it('exposes formSectionVariants for consumer composition', () => {
    expect(formSectionVariants({ spacing: 'compact' })).toContain('gap-3')
    expect(formSectionVariants({ spacing: 'default' })).toContain('gap-4')
  })
})

describe('FormGrid (PR 2.1)', () => {
  it('renders a 2-column responsive grid by default', () => {
    const wrapper = mount(FormGrid, {
      slots: { default: '<div data-testid="cell">cell</div>' },
    })

    const root = wrapper.find('[data-slot="form-grid"]')
    expect(root.classes()).toContain('grid')
    expect(root.classes()).toContain('grid-cols-1')
    expect(root.classes()).toContain('md:grid-cols-2')
    expect(root.classes()).toContain('gap-4')
    expect(root.find('[data-testid="cell"]').exists()).toBe(true)
  })

  it('switches column count via the cols variant', () => {
    const wrapper = mount(FormGrid, { props: { cols: '3' } })
    const classes = wrapper.classes()
    expect(classes).toContain('sm:grid-cols-2')
    expect(classes).toContain('lg:grid-cols-3')
  })

  it('switches gap size via the gap variant', () => {
    const wrapper = mount(FormGrid, { props: { gap: 'lg' } })
    expect(wrapper.classes()).toContain('gap-6')
  })

  it('exposes formGridVariants for consumer composition', () => {
    expect(formGridVariants({ cols: '4' })).toContain('lg:grid-cols-4')
  })
})

describe('FormFooter (PR 2.1)', () => {
  it('renders an end-aligned footer with a top border by default', () => {
    const wrapper = mount(FormFooter, {
      slots: { default: '<button>Save</button>' },
    })

    const root = wrapper.find('[data-slot="form-footer"]')
    expect(root.element.tagName).toBe('FOOTER')
    expect(root.classes()).toContain('justify-end')
    expect(root.classes()).toContain('border-t')
    expect(root.classes()).toContain('pt-4')
    expect(root.html()).toContain('<button>Save</button>')
  })

  it('omits the top border when border="none"', () => {
    const wrapper = mount(FormFooter, { props: { border: 'none' } })
    expect(wrapper.classes()).not.toContain('border-t')
    expect(wrapper.classes()).not.toContain('pt-4')
  })

  it('applies the alignment variant', () => {
    const wrapper = mount(FormFooter, { props: { align: 'between' } })
    expect(wrapper.classes()).toContain('justify-between')
  })

  it('exposes formFooterVariants for consumer composition', () => {
    expect(formFooterVariants({ align: 'start', border: 'top' })).toContain(
      'justify-start',
    )
  })
})
