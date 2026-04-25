import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'

import {
  FilterBar,
  SearchInput,
  filterBarVariants,
} from '@design/patterns'

/**
 * PR 2.3 — SearchInput + FilterBar.
 *
 * SearchInput is a decoration around `<Input>` (leading icon + clear
 * button). The pattern is intentionally dumb: views own the debounce
 * via `useDebouncedSearch` composable, this primitive only handles
 * the visual contract and the clear interaction.
 *
 * FilterBar is a slot-based toolbar that pins a search slot left,
 * filters in the middle, and actions right. Tests here verify the
 * slot wiring and the density variant — actual filter chips are
 * caller-owned and exercised by the views in Phases 3/4.
 */
describe('SearchInput (PR 2.3)', () => {
  it('renders a searchbox role and is two-way bound via v-model', async () => {
    const Host = defineComponent({
      components: { SearchInput },
      setup() {
        const query = ref('')
        return { query }
      },
      template: `<SearchInput v-model="query" placeholder="Find" />`,
    })

    const wrapper = mount(Host)
    const input = wrapper.find('input[role="searchbox"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('placeholder')).toBe('Find')
    expect(input.attributes('aria-label')).toBe('Search')

    await input.setValue('camera')
    expect(wrapper.vm.query).toBe('camera')
  })

  it('hides the clear button while empty and shows it once a value exists', async () => {
    const Host = defineComponent({
      components: { SearchInput },
      setup() {
        const query = ref('')
        return { query }
      },
      template: `<SearchInput v-model="query" />`,
    })

    const wrapper = mount(Host)
    expect(wrapper.find('[data-testid="search-input-clear"]').exists()).toBe(false)

    await wrapper.find('input').setValue('x')
    expect(wrapper.find('[data-testid="search-input-clear"]').exists()).toBe(true)
  })

  it('clear button resets the value and emits the clear event', async () => {
    const Host = defineComponent({
      components: { SearchInput },
      emits: ['cleared'],
      setup(_, { emit }) {
        const query = ref('camera')
        const onClear = () => emit('cleared')
        return { query, onClear }
      },
      template: `<SearchInput v-model="query" @clear="onClear" />`,
    })

    const wrapper = mount(Host)
    await wrapper.get('[data-testid="search-input-clear"]').trigger('click')

    expect(wrapper.vm.query).toBe('')
    expect(wrapper.emitted('cleared')).toBeTruthy()
  })

  it('exposes an aria-label override for non-default search contexts', () => {
    const wrapper = mount(SearchInput, {
      props: { ariaLabel: 'Search files', modelValue: '' },
    })

    expect(wrapper.find('input').attributes('aria-label')).toBe('Search files')
  })
})

describe('FilterBar (PR 2.3)', () => {
  it('renders the search/default/actions slots into their respective regions', () => {
    const wrapper = mount(FilterBar, {
      slots: {
        search: '<div data-testid="search">search</div>',
        default: '<div data-testid="filter">filter</div>',
        actions: '<div data-testid="actions">actions</div>',
      },
    })

    const searchRegion = wrapper.find('[data-slot="filter-bar-search"]')
    const filtersRegion = wrapper.find('[data-slot="filter-bar-filters"]')
    const actionsRegion = wrapper.find('[data-slot="filter-bar-actions"]')

    expect(searchRegion.find('[data-testid="search"]').exists()).toBe(true)
    expect(filtersRegion.find('[data-testid="filter"]').exists()).toBe(true)
    expect(actionsRegion.find('[data-testid="actions"]').exists()).toBe(true)
  })

  it('omits empty regions to keep the toolbar gap-tight', () => {
    const wrapper = mount(FilterBar, {
      slots: { default: '<div>only filters</div>' },
    })

    expect(wrapper.find('[data-slot="filter-bar-search"]').exists()).toBe(false)
    expect(wrapper.find('[data-slot="filter-bar-actions"]').exists()).toBe(false)
    expect(wrapper.find('[data-slot="filter-bar-filters"]').exists()).toBe(true)
  })

  it('exposes the toolbar role and applies the density variant', () => {
    const wrapper = mount(FilterBar, {
      props: { density: 'compact' },
      slots: { default: '<span>x</span>' },
    })

    const root = wrapper.find('[data-slot="filter-bar"]')
    expect(root.attributes('role')).toBe('toolbar')
    expect(root.classes()).toContain('p-1.5')
    expect(root.classes()).toContain('gap-1.5')
  })

  it('exposes filterBarVariants for consumer composition', () => {
    expect(filterBarVariants({ density: 'relaxed' })).toContain('p-3')
  })
})
