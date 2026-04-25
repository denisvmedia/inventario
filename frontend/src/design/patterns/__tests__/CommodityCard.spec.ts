import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CommodityCard from '../CommodityCard.vue'

const baseCommodity = {
  id: 'c-1',
  attributes: {
    name: 'MacBook Pro',
    type: 'electronics',
    status: 'in_use',
    draft: false,
    count: 1,
    area_id: 'area-1',
    purchase_date: '2024-03-15',
    original_price: '2499.00',
    original_price_currency: 'USD',
    converted_original_price: '2499.00',
    current_price: '2200.00',
  },
}

describe('CommodityCard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders the name, type label, formatted price, and status pill', () => {
    const wrapper = mount(CommodityCard, { props: { commodity: baseCommodity } })
    expect(wrapper.text()).toContain('MacBook Pro')
    expect(wrapper.text()).toContain('Electronics')
    expect(wrapper.text()).toContain('2200.00')
    expect(wrapper.text()).toContain('In use')
  })

  it('preserves the legacy `commodity-card` class anchor', () => {
    const wrapper = mount(CommodityCard, { props: { commodity: baseCommodity } })
    expect(wrapper.find('.commodity-card').exists()).toBe(true)
  })

  it('shows count and per-unit price when count > 1', () => {
    const c = {
      ...baseCommodity,
      attributes: { ...baseCommodity.attributes, count: 4 },
    }
    const wrapper = mount(CommodityCard, { props: { commodity: c } })
    expect(wrapper.text()).toContain('×4')
    expect(wrapper.text()).toContain('per unit')
  })

  it('hides secondary metadata in compact mode', () => {
    const wrapper = mount(CommodityCard, {
      props: { commodity: baseCommodity, compact: true },
    })
    expect(wrapper.text()).not.toContain('Electronics')
    expect(wrapper.text()).not.toContain('per unit')
    // Status pill is still shown in compact mode.
    expect(wrapper.text()).toContain('In use')
  })

  it('applies the `draft` modifier and shows the Draft pill when draft=true', () => {
    const c = {
      ...baseCommodity,
      attributes: { ...baseCommodity.attributes, draft: true },
    }
    const wrapper = mount(CommodityCard, { props: { commodity: c } })
    expect(wrapper.find('.commodity-card.draft').exists()).toBe(true)
    expect(wrapper.text()).toContain('Draft')
  })

  it('renders the location label when showLocation is true and maps are provided', () => {
    const wrapper = mount(CommodityCard, {
      props: {
        commodity: baseCommodity,
        showLocation: true,
        areaMap: { 'area-1': { name: 'Office', locationId: 'loc-1' } },
        locationMap: { 'loc-1': { name: 'Home' } },
      },
    })
    expect(wrapper.text()).toContain('Home / Office')
  })

  it('emits view on click and edit/delete from the trailing buttons', async () => {
    const wrapper = mount(CommodityCard, { props: { commodity: baseCommodity } })
    await wrapper.trigger('click')
    await wrapper.get('[data-testid="commodity-card-c-1-edit"]').trigger('click')
    await wrapper.get('[data-testid="commodity-card-c-1-delete"]').trigger('click')
    expect(wrapper.emitted('view')).toEqual([['c-1']])
    expect(wrapper.emitted('edit')).toEqual([['c-1']])
    expect(wrapper.emitted('delete')).toEqual([['c-1']])
  })

  it('marks the card as highlighted when ids match', () => {
    const wrapper = mount(CommodityCard, {
      props: { commodity: baseCommodity, highlightCommodityId: 'c-1' },
    })
    expect(wrapper.find('.commodity-card.highlighted').exists()).toBe(true)
  })
})
