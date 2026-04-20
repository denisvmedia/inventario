import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import IconPicker from '../IconPicker.vue'
import {
  GROUP_ICONS,
  GROUP_ICON_CATEGORIES,
  GROUP_ICON_CATEGORY_STORAGE,
} from '@/constants/groupIcons'

function findOptionByEmoji(wrapper: ReturnType<typeof mount<typeof IconPicker>>, emoji: string) {
  return wrapper
    .findAll<HTMLButtonElement>('.icon-picker__icon')
    .find((btn) => btn.text() === emoji)
}

function create(modelValue = '') {
  return mount(IconPicker, {
    props: {
      modelValue,
      'onUpdate:modelValue': () => {
        // not wired to v-model in the test; we assert on emitted events
      },
    },
  })
}

describe('IconPicker.vue', () => {
  it('renders a closed trigger with a placeholder when no icon is selected', () => {
    const wrapper = create('')
    expect(wrapper.find('[data-testid="icon-picker-panel"]').exists()).toBe(false)
    const trigger = wrapper.find('[data-testid="icon-picker-trigger"]')
    expect(trigger.exists()).toBe(true)
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('opens the panel on trigger click', async () => {
    const wrapper = create('')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    expect(wrapper.find('[data-testid="icon-picker-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="icon-picker-trigger"]').attributes('aria-expanded')).toBe(
      'true',
    )
  })

  it('shows icons from the default category on first open', async () => {
    const wrapper = create('')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    const firstCategory = GROUP_ICON_CATEGORIES[0]
    const expected = GROUP_ICONS.filter((i) => i.category === firstCategory.id)
    const buttons = wrapper.findAll('.icon-picker__icon')
    expect(buttons.length).toBe(expected.length)
  })

  it('switches categories when a tab is clicked', async () => {
    const wrapper = create('')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    await wrapper.find(`[data-testid="icon-picker-tab-${GROUP_ICON_CATEGORY_STORAGE}"]`).trigger('click')
    const expected = GROUP_ICONS.filter((i) => i.category === GROUP_ICON_CATEGORY_STORAGE)
    const buttons = wrapper.findAll('.icon-picker__icon')
    expect(buttons.length).toBe(expected.length)
  })

  it('emits update:modelValue when an icon is picked', async () => {
    const wrapper = create('')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    await nextTick()
    const target = GROUP_ICONS[0]
    const option = findOptionByEmoji(wrapper, target.emoji)
    expect(option, `expected to find option for ${target.emoji}`).toBeTruthy()
    await option!.trigger('click')
    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect(emitted![0]).toEqual([target.emoji])
  })

  it('emits an empty string and closes when "No icon" is clicked', async () => {
    const wrapper = create('📦')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    await wrapper.find('[data-testid="icon-picker-clear"]').trigger('click')
    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect(emitted![0]).toEqual([''])
    expect(wrapper.find('[data-testid="icon-picker-panel"]').exists()).toBe(false)
  })

  it('jumps to the category of the current value when opened', async () => {
    // 📦 lives in the storage category.
    const wrapper = create('📦')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    const storageTab = wrapper.find(
      `[data-testid="icon-picker-tab-${GROUP_ICON_CATEGORY_STORAGE}"]`,
    )
    expect(storageTab.attributes('aria-selected')).toBe('true')
  })

  it('marks the currently selected icon aria-pressed', async () => {
    const wrapper = create('📦')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    await nextTick()
    const selected = findOptionByEmoji(wrapper, '📦')
    expect(selected, 'expected to find option for 📦').toBeTruthy()
    expect(selected!.attributes('aria-pressed')).toBe('true')
  })

  it('disables the "No icon" button when the value is already empty', async () => {
    const wrapper = create('')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    const clear = wrapper.find('[data-testid="icon-picker-clear"]')
    expect((clear.element as HTMLButtonElement).disabled).toBe(true)
  })

  it('closes the panel when "Done" is clicked', async () => {
    const wrapper = create('📦')
    await wrapper.find('[data-testid="icon-picker-trigger"]').trigger('click')
    expect(wrapper.find('[data-testid="icon-picker-panel"]').exists()).toBe(true)
    await wrapper.find('[data-testid="icon-picker-close"]').trigger('click')
    expect(wrapper.find('[data-testid="icon-picker-panel"]').exists()).toBe(false)
  })
})
