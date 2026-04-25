import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'

import { InlineListEditor } from '@design/patterns'

/**
 * PR 2.2 — InlineListEditor.
 *
 * Generic add/remove/update wrapper for variable-length value lists
 * (the "Add Tag", "Add URL", "Add Serial Number" patterns currently
 * scattered across the legacy commodity form).
 *
 * Tests verify the editor's contract:
 *   - default `<Input>` rendering for the common `string[]` case,
 *   - add appends a new item using `newItem` factory or a blank string,
 *   - remove drops the row at the given index,
 *   - the `item` slot replaces the default row renderer and receives
 *     the row update callback,
 *   - `allowEmpty: false` keeps at least one row and disables the
 *     remove button on the last item.
 */
describe('InlineListEditor (PR 2.2)', () => {
  it('renders an Input per item in the model and v-model writes back on edit', async () => {
    const Host = defineComponent({
      components: { InlineListEditor },
      setup() {
        const items = ref<string[]>(['alpha', 'beta'])
        return { items }
      },
      template: `
        <InlineListEditor
          v-model="items"
          add-label="Add Tag"
          placeholder="Enter a tag"
        />
      `,
    })

    const wrapper = mount(Host)
    const inputs = wrapper.findAll('input')
    expect(inputs).toHaveLength(2)
    expect((inputs[0].element as HTMLInputElement).value).toBe('alpha')

    await inputs[0].setValue('omega')
    expect(wrapper.vm.items).toEqual(['omega', 'beta'])
  })

  it('adds a new empty item when the add button is clicked', async () => {
    const Host = defineComponent({
      components: { InlineListEditor },
      setup() {
        const items = ref<string[]>([])
        return { items }
      },
      template: `
        <InlineListEditor v-model="items" add-label="Add URL" />
      `,
    })

    const wrapper = mount(Host)
    expect(wrapper.findAll('input')).toHaveLength(0)

    await wrapper.get('[data-testid="inline-list-editor-add"]').trigger('click')
    expect(wrapper.vm.items).toEqual([''])

    await wrapper.get('[data-testid="inline-list-editor-add"]').trigger('click')
    expect(wrapper.vm.items).toEqual(['', ''])
  })

  it('uses the newItem factory when provided', async () => {
    interface TagRow {
      label: string
      color: string
    }

    const Host = defineComponent({
      components: { InlineListEditor },
      setup() {
        const items = ref<TagRow[]>([])
        const factory = (): TagRow => ({ label: '', color: 'gray' })
        return { items, factory }
      },
      template: `
        <InlineListEditor
          v-model="items"
          add-label="Add Tag"
          :new-item="factory"
        >
          <template #item="{ item }">
            <span data-testid="row">{{ item.color }}</span>
          </template>
        </InlineListEditor>
      `,
    })

    const wrapper = mount(Host)
    await wrapper.get('[data-testid="inline-list-editor-add"]').trigger('click')

    expect(wrapper.vm.items).toEqual([{ label: '', color: 'gray' }])
    expect(wrapper.get('[data-testid="row"]').text()).toBe('gray')
  })

  it('removes the row at the given index', async () => {
    const Host = defineComponent({
      components: { InlineListEditor },
      setup() {
        const items = ref<string[]>(['a', 'b', 'c'])
        return { items }
      },
      template: `
        <InlineListEditor v-model="items" add-label="Add" />
      `,
    })

    const wrapper = mount(Host)
    const removeButtons = wrapper.findAll('[data-testid="inline-list-editor-remove"]')
    expect(removeButtons).toHaveLength(3)

    await removeButtons[1].trigger('click')
    expect(wrapper.vm.items).toEqual(['a', 'c'])
  })

  it('disables removal of the last row when allowEmpty=false', async () => {
    const Host = defineComponent({
      components: { InlineListEditor },
      setup() {
        const items = ref<string[]>(['only'])
        return { items }
      },
      template: `
        <InlineListEditor
          v-model="items"
          add-label="Add"
          :allow-empty="false"
        />
      `,
    })

    const wrapper = mount(Host)
    const remove = wrapper.get('[data-testid="inline-list-editor-remove"]')
    expect((remove.element as HTMLButtonElement).disabled).toBe(true)

    await remove.trigger('click')
    expect(wrapper.vm.items).toEqual(['only'])
  })

  it('renders the item slot in place of the default Input and exposes the row update callback', async () => {
    const Host = defineComponent({
      components: { InlineListEditor },
      setup() {
        const items = ref<string[]>(['x'])
        return { items }
      },
      render() {
        return h(
          InlineListEditor,
          {
            modelValue: this.items,
            'onUpdate:modelValue': (value: string[]) => {
              this.items = value
            },
            addLabel: 'Add',
          },
          {
            item: ({
              item,
              update,
            }: {
              item: string
              update: (value: string) => void
            }) =>
              h(
                'button',
                {
                  type: 'button',
                  'data-testid': 'custom-row',
                  onClick: () => update(`${item}!`),
                },
                item,
              ),
          },
        )
      },
    })

    const wrapper = mount(Host)
    const row = wrapper.get('[data-testid="custom-row"]')
    expect(row.text()).toBe('x')
    await row.trigger('click')
    expect(wrapper.vm.items).toEqual(['x!'])
  })

  it('renders the empty slot when the list is empty', () => {
    const wrapper = mount(InlineListEditor, {
      props: {
        modelValue: [] as string[],
        addLabel: 'Add Tag',
      },
      slots: {
        empty: '<p data-testid="empty">No tags yet.</p>',
      },
    })

    expect(wrapper.find('[data-testid="empty"]').text()).toBe('No tags yet.')
  })
})
