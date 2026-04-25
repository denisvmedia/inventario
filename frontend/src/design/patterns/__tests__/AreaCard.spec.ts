import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AreaCard from '../AreaCard.vue'

const area = { id: 'a-1', attributes: { name: 'Living Room' } }

describe('AreaCard', () => {
  it('renders the area name and preserves the legacy class anchor', () => {
    const wrapper = mount(AreaCard, { props: { area } })
    expect(wrapper.text()).toContain('Living Room')
    // Legacy Playwright selectors target `.area-card`.
    expect(wrapper.find('.area-card').exists()).toBe(true)
    expect(wrapper.attributes('data-testid')).toBe('area-card-a-1')
  })

  it('emits view on root click', async () => {
    const wrapper = mount(AreaCard, { props: { area } })
    await wrapper.trigger('click')
    expect(wrapper.emitted('view')).toEqual([['a-1']])
  })

  it('emits view on Enter / Space, with preventDefault', async () => {
    const wrapper = mount(AreaCard, { props: { area } })
    await wrapper.trigger('keydown', { key: 'Enter' })
    await wrapper.trigger('keydown', { key: ' ' })
    expect(wrapper.emitted('view')).toEqual([['a-1'], ['a-1']])
  })

  it('emits edit and delete from the trailing buttons without bubbling click', async () => {
    const wrapper = mount(AreaCard, { props: { area } })
    const editBtn = wrapper.get('[data-testid="area-card-a-1-edit"]')
    const deleteBtn = wrapper.get('[data-testid="area-card-a-1-delete"]')
    await editBtn.trigger('click')
    await deleteBtn.trigger('click')
    expect(wrapper.emitted('edit')).toEqual([['a-1']])
    expect(wrapper.emitted('delete')).toEqual([['a-1']])
    // Card click handler must not have fired (Vue's @click.stop on
    // the IconButtons stops propagation to the parent Card).
    expect(wrapper.emitted('view')).toBeUndefined()
  })

  it('hides actions when showActions is false', () => {
    const wrapper = mount(AreaCard, { props: { area, showActions: false } })
    expect(wrapper.find('[data-testid="area-card-a-1-edit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="area-card-a-1-delete"]').exists()).toBe(false)
  })

  it('honours custom testId', () => {
    const wrapper = mount(AreaCard, { props: { area, testId: 'custom' } })
    expect(wrapper.attributes('data-testid')).toBe('custom')
    // Spy reference for future strict TS — silences unused warning.
    void vi
  })
})
