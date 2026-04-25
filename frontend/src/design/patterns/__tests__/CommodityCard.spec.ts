import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import CommodityCard from '../CommodityCard.vue'

const baseProps = {
  name: 'Coffee Maker',
  type: 'electronics',
  status: 'in_use' as const,
}

describe('CommodityCard', () => {
  it('renders the name in an <h3>', () => {
    const wrapper = mount(CommodityCard, { props: baseProps })

    expect(wrapper.get('h3').text()).toBe('Coffee Maker')
  })

  it('keeps the .commodity-card legacy anchor on the outermost element', () => {
    const wrapper = mount(CommodityCard, { props: baseProps })

    expect(wrapper.classes()).toContain('commodity-card')
  })

  it('forwards testId to the outermost element', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, testId: 'comm-1' },
    })

    expect(wrapper.attributes('data-testid')).toBe('comm-1')
  })

  it('renders the location/area breadcrumb when provided', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, locationName: 'Office', areaName: 'Kitchen' },
    })

    expect(wrapper.text()).toContain('Office / Kitchen')
  })

  it('renders only the location when no area is given', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, locationName: 'Office' },
    })

    expect(wrapper.text()).toContain('Office')
    expect(wrapper.text()).not.toContain('/')
  })

  it('renders the type label for known commodity types', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, type: 'furniture' },
    })

    expect(wrapper.text()).toContain('Furniture')
  })

  it('falls back to the raw type id for unknown types', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, type: 'made_up' },
    })

    expect(wrapper.text()).toContain('made_up')
  })

  it('shows ×N count only when count > 1', () => {
    const single = mount(CommodityCard, { props: { ...baseProps, count: 1 } })
    expect(single.text()).not.toContain('×')

    const many = mount(CommodityCard, { props: { ...baseProps, count: 4 } })
    expect(many.text()).toContain('×4')
  })

  it('renders the formatted purchase date', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, purchaseDate: '2026-03-05' },
    })

    expect(wrapper.text()).toMatch(/Mar.+5/)
  })

  it('renders the displayPrice and per-unit price when count > 1', () => {
    const wrapper = mount(CommodityCard, {
      props: {
        ...baseProps,
        count: 3,
        displayPrice: '300.00 USD',
        pricePerUnit: '100.00 USD',
      },
    })

    expect(wrapper.text()).toContain('300.00 USD')
    expect(wrapper.text()).toContain('100.00 USD per unit')
  })

  it('omits per-unit price when count === 1', () => {
    const wrapper = mount(CommodityCard, {
      props: {
        ...baseProps,
        count: 1,
        displayPrice: '100.00 USD',
        pricePerUnit: '100.00 USD',
      },
    })

    expect(wrapper.text()).toContain('100.00 USD')
    expect(wrapper.text()).not.toContain('per unit')
  })

  it('shows the status pill with the commodity status', () => {
    const wrapper = mount(CommodityCard, { props: { ...baseProps, status: 'sold' } })

    expect(wrapper.attributes('data-status')).toBe('sold')
    expect(wrapper.text()).toContain('Sold')
  })

  it('overrides the status to draft when the draft flag is set', () => {
    const wrapper = mount(CommodityCard, {
      props: { ...baseProps, status: 'in_use', draft: true },
    })

    expect(wrapper.attributes('data-status')).toBe('draft')
    expect(wrapper.text()).toContain('Draft')
  })

  it('applies the matching border-l-status-* class for each status', () => {
    const cases: Array<[CommodityCardStatus, string]> = [
      ['in_use', 'border-l-status-in-use'],
      ['sold', 'border-l-status-sold'],
      ['lost', 'border-l-status-lost'],
      ['disposed', 'border-l-status-disposed'],
      ['written_off', 'border-l-status-written-off'],
    ]
    for (const [status, expected] of cases) {
      const w = mount(CommodityCard, { props: { ...baseProps, status } })
      expect(w.classes()).toContain(expected)
    }

    const draft = mount(CommodityCard, { props: { ...baseProps, draft: true } })
    expect(draft.classes()).toContain('border-l-status-draft')
  })

  it('emits view on click and on Enter / Space', async () => {
    const wrapper = mount(CommodityCard, { props: baseProps })

    await wrapper.trigger('click')
    await wrapper.trigger('keydown', { key: 'Enter' })
    await wrapper.trigger('keydown', { key: ' ' })

    expect(wrapper.emitted('view')).toHaveLength(3)
  })

  it('emits edit and delete from the action buttons without bubbling view', async () => {
    const wrapper = mount(CommodityCard, { props: baseProps })

    await wrapper.get('[aria-label="Edit commodity"]').trigger('click')
    await wrapper.get('[aria-label="Delete commodity"]').trigger('click')

    expect(wrapper.emitted('edit')).toHaveLength(1)
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('view')).toBeUndefined()
  })

  it('exposes legacy title attributes on the action buttons', () => {
    const wrapper = mount(CommodityCard, { props: baseProps })

    expect(wrapper.get('[aria-label="Edit commodity"]').attributes('title')).toBe('Edit')
    expect(wrapper.get('[aria-label="Delete commodity"]').attributes('title')).toBe('Delete')
  })

  it('renders an accessible role/tabindex on the card root', () => {
    const wrapper = mount(CommodityCard, { props: baseProps })

    expect(wrapper.attributes('role')).toBe('button')
    expect(wrapper.attributes('tabindex')).toBe('0')
  })
})

type CommodityCardStatus =
  | 'in_use'
  | 'sold'
  | 'lost'
  | 'disposed'
  | 'written_off'
