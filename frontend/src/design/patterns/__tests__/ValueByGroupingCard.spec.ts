import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'

import ValueByGroupingCard, {
  type ValueByGroupingItem,
} from '../ValueByGroupingCard.vue'

const items: ValueByGroupingItem[] = [
  { id: 'a', name: 'Office', value: '100.00 USD' },
  { id: 'b', name: 'Garage', value: '250.00 USD' },
]

describe('ValueByGroupingCard', () => {
  it('renders the title in an <h3>', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'Value by Location', items },
    })

    expect(wrapper.get('h3').text()).toBe('Value by Location')
  })

  it('renders each item with name and value', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'X', items },
    })

    const rows = wrapper.findAll('li')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('Office')
    expect(rows[0].text()).toContain('100.00 USD')
    expect(rows[1].text()).toContain('Garage')
    expect(rows[1].text()).toContain('250.00 USD')
  })

  it('falls back to name when id is missing for v-for keys', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'X', items: [{ name: 'Lone', value: '1.00' }] },
    })

    expect(wrapper.text()).toContain('Lone')
  })

  it('renders skeleton rows when loading', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'X', items, loading: true, skeletonRows: 4 },
    })

    expect(wrapper.attributes('data-testid')).toBeUndefined()
    const placeholderList = wrapper.get('[aria-busy="true"]')
    expect(placeholderList.findAll('li')).toHaveLength(4)
    expect(wrapper.text()).not.toContain('Office')
  })

  it('renders the empty copy when items is [] and not loading', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'X', items: [], empty: 'Nothing yet' },
    })

    expect(wrapper.text()).toContain('Nothing yet')
  })

  it('renders the actions slot when provided', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'X', items },
      slots: { actions: () => h('a', { class: 'view-all' }, 'View all') },
    })

    expect(wrapper.find('.view-all').exists()).toBe(true)
  })

  it('forwards testId to the section root', () => {
    const wrapper = mount(ValueByGroupingCard, {
      props: { title: 'X', items, testId: 'value-by-location' },
    })

    expect(wrapper.attributes('data-testid')).toBe('value-by-location')
  })
})
