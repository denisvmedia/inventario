import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BulkActionsBar from '../BulkActionsBar.vue'

describe('BulkActionsBar', () => {
  it('renders nothing when count is 0', () => {
    const wrapper = mount(BulkActionsBar, { props: { count: 0 } })
    expect(wrapper.find('[data-testid="bulk-actions-bar"]').exists()).toBe(false)
  })

  it('renders the bar and the count label when count > 0', () => {
    const wrapper = mount(BulkActionsBar, { props: { count: 3 } })
    const bar = wrapper.get('[data-testid="bulk-actions-bar"]')
    expect(bar.exists()).toBe(true)
    expect(wrapper.get('[data-testid="bulk-actions-count"]').text()).toContain('3')
  })

  it('uses the singular noun when count is 1', () => {
    const wrapper = mount(BulkActionsBar, {
      props: { count: 1, itemNoun: 'commodity', itemNounPlural: 'commodities' },
    })
    expect(wrapper.get('[data-testid="bulk-actions-count"]').text()).toContain(
      '1 commodity selected',
    )
  })

  it('uses the plural noun when count > 1', () => {
    const wrapper = mount(BulkActionsBar, {
      props: { count: 5, itemNoun: 'commodity', itemNounPlural: 'commodities' },
    })
    expect(wrapper.get('[data-testid="bulk-actions-count"]').text()).toContain(
      '5 commodities selected',
    )
  })

  it('emits clear when the deselect-all button is clicked', async () => {
    const wrapper = mount(BulkActionsBar, { props: { count: 2 } })
    await wrapper.get('[data-testid="bulk-actions-clear"]').trigger('click')
    expect(wrapper.emitted('clear')).toHaveLength(1)
  })

  it('renders default slot content', () => {
    const wrapper = mount(BulkActionsBar, {
      props: { count: 1 },
      slots: {
        default: '<button data-testid="bulk-action-foo">Foo</button>',
      },
    })
    expect(wrapper.find('[data-testid="bulk-action-foo"]').exists()).toBe(true)
  })
})
