import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import AppConfirmDialog from '../AppConfirmDialog.vue'

describe('AppConfirmDialog', () => {
  it('renders title + message + labels when open', async () => {
    const wrapper = mount(AppConfirmDialog, {
      props: {
        open: true,
        title: 'Delete Foo',
        message: '<strong>Foo</strong> will be removed',
        confirmLabel: 'Delete',
        cancelLabel: 'Keep',
      },
      attachTo: document.body,
    })

    await nextTick()

    const body = document.body.innerHTML
    expect(body).toContain('Delete Foo')
    expect(body).toContain('<strong>Foo</strong>')
    expect(body).toContain('Delete')
    expect(body).toContain('Keep')
    wrapper.unmount()
  })

  it('emits confirm when the confirm button is clicked', async () => {
    const wrapper = mount(AppConfirmDialog, {
      props: { open: true, confirmLabel: 'Delete' },
      attachTo: document.body,
    })

    await nextTick()

    // The AlertDialogAction renders into a teleported portal, so use
    // document.body to find the button and dispatch a real click.
    const buttons = Array.from(document.body.querySelectorAll('button'))
    const confirmButton = buttons.find((b) => b.textContent?.trim() === 'Delete')
    expect(confirmButton).toBeTruthy()
    confirmButton!.click()
    await nextTick()

    expect(wrapper.emitted('confirm')).toHaveLength(1)
    wrapper.unmount()
  })

  it('emits cancel when the cancel button is clicked', async () => {
    const wrapper = mount(AppConfirmDialog, {
      props: { open: true, cancelLabel: 'No thanks' },
      attachTo: document.body,
    })

    await nextTick()

    const buttons = Array.from(document.body.querySelectorAll('button'))
    const cancel = buttons.find((b) => b.textContent?.trim() === 'No thanks')
    expect(cancel).toBeTruthy()
    cancel!.click()
    await nextTick()

    expect(wrapper.emitted('cancel')).toHaveLength(1)
    wrapper.unmount()
  })
})
