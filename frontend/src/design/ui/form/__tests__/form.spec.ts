import { describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ExampleForm from '../__examples__/ExampleForm.vue'
import { exampleFormSchema } from '../__examples__/ExampleForm.schema'

/**
 * PR 0.6 sanity check: verifies the vee-validate + zod + shadcn-vue
 * `<Form>` pipeline prescribed by devdocs/frontend/forms.md is wired
 * correctly. We exercise it against the canonical ExampleForm so any
 * future regression in the stack (e.g. a Reka UI upgrade that changes
 * the FormField slot contract, or a zod major that alters inference)
 * trips the test.
 */
describe('zod schema (ExampleForm)', () => {
  it('accepts a well-formed payload', () => {
    const parsed = exampleFormSchema.safeParse({
      name: 'Camera',
      email: 'owner@example.com',
      count: 2,
    })
    expect(parsed.success).toBe(true)
  })

  it('rejects a missing name with the schema message', () => {
    const parsed = exampleFormSchema.safeParse({
      name: '',
      email: 'owner@example.com',
      count: 2,
    })
    expect(parsed.success).toBe(false)
    if (!parsed.success) {
      const nameIssue = parsed.error.issues.find((i) => i.path[0] === 'name')
      expect(nameIssue?.message).toBe('Name is required')
    }
  })

  it('coerces numeric strings to numbers', () => {
    const parsed = exampleFormSchema.safeParse({
      name: 'Camera',
      email: 'owner@example.com',
      count: '3',
    })
    expect(parsed.success).toBe(true)
    if (parsed.success) {
      expect(parsed.data.count).toBe(3)
      expect(typeof parsed.data.count).toBe('number')
    }
  })
})

describe('ExampleForm (vee-validate + zod + shadcn <Form>)', () => {
  it('renders three FormField groups with FormLabel + Input', () => {
    const wrapper = mount(ExampleForm)
    const labels = wrapper.findAll('[data-slot="form-label"]')
    expect(labels).toHaveLength(3)
    expect(labels.map((l) => l.text())).toEqual(['Name', 'Email', 'Count'])
    expect(wrapper.findAll('input')).toHaveLength(3)
  })

  it('marks required labels via data-required (renders the asterisk affordance)', () => {
    const wrapper = mount(ExampleForm)
    const labels = wrapper.findAll('[data-slot="form-label"]')
    for (const label of labels) {
      expect(label.attributes('data-required')).toBe('')
    }
  })

  it('blocks submission and surfaces the expected field errors on invalid input', async () => {
    const wrapper = mount(ExampleForm)
    const exposed = wrapper.vm as unknown as {
      onSubmit: (_e?: Event) => Promise<void>
      errors: Record<string, string | undefined>
    }

    await exposed.onSubmit()
    await flushPromises()

    expect(wrapper.emitted('submitted')).toBeUndefined()
    expect(exposed.errors.name).toBe('Name is required')
    expect(exposed.errors.email).toBeTruthy()
    expect(exposed.errors.count).toBeTruthy()
  })

  it('emits a typed payload with coerced count on a valid submission', async () => {
    const wrapper = mount(ExampleForm)
    const exposed = wrapper.vm as unknown as {
      values: Record<string, unknown>
      onSubmit: (_e?: Event) => Promise<void>
    }
    const [nameEl, emailEl, countEl] = wrapper.findAll('input')

    await nameEl.setValue('Camera')
    await emailEl.setValue('owner@example.com')
    await countEl.setValue('4')
    await flushPromises()

    expect(exposed.values).toEqual({ name: 'Camera', email: 'owner@example.com', count: 4 })

    await exposed.onSubmit()
    await flushPromises()

    const submitted = wrapper.emitted('submitted')
    expect(submitted).toBeTruthy()
    expect(submitted?.[0]?.[0]).toEqual({
      name: 'Camera',
      email: 'owner@example.com',
      count: 4,
    })
  })
})
