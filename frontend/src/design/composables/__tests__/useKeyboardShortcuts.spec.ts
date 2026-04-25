import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'

import { useKeyboardShortcuts } from '../useKeyboardShortcuts'

const Host = defineComponent({
  props: {
    bindings: {
      type: Array as () => Parameters<typeof useKeyboardShortcuts>[0],
      required: true,
    },
  },
  setup(props) {
    useKeyboardShortcuts(props.bindings)
    return () => h('div')
  },
})

function dispatch(opts: KeyboardEventInit & { key: string; target?: EventTarget }) {
  const event = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...opts })
  if (opts.target) {
    Object.defineProperty(event, 'target', { value: opts.target })
  }
  window.dispatchEvent(event)
  return event
}

describe('useKeyboardShortcuts', () => {
  let originalNavigator: typeof globalThis.navigator | undefined
  beforeEach(() => {
    originalNavigator = globalThis.navigator
  })
  afterEach(() => {
    if (originalNavigator !== undefined) {
      Object.defineProperty(globalThis, 'navigator', {
        configurable: true,
        value: originalNavigator,
      })
    }
  })

  function setPlatform(platform: string) {
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: { platform, userAgent: '' },
    })
  }

  it('fires the handler when the matching key + modifier are pressed', () => {
    setPlatform('MacIntel')
    const handler = vi.fn()
    mount(Host, {
      props: { bindings: [{ key: 'k', modifiers: ['mod'], handler }] },
    })

    dispatch({ key: 'k', metaKey: true })
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('treats `mod` as Ctrl on non-mac platforms', () => {
    setPlatform('Win32')
    const handler = vi.fn()
    mount(Host, {
      props: { bindings: [{ key: 'k', modifiers: ['mod'], handler }] },
    })

    dispatch({ key: 'k', metaKey: true })
    expect(handler).not.toHaveBeenCalled()

    dispatch({ key: 'k', ctrlKey: true })
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('does not fire when modifier set differs', () => {
    setPlatform('MacIntel')
    const handler = vi.fn()
    mount(Host, {
      props: { bindings: [{ key: 'k', modifiers: ['mod'], handler }] },
    })

    dispatch({ key: 'k' })
    dispatch({ key: 'k', altKey: true, metaKey: true })
    expect(handler).not.toHaveBeenCalled()
  })

  it('honours ignoreInInput for INPUT and contenteditable targets', () => {
    setPlatform('MacIntel')
    const handler = vi.fn()
    mount(Host, {
      props: {
        bindings: [{ key: 'k', modifiers: ['mod'], handler, ignoreInInput: true }],
      },
    })

    const input = document.createElement('input')
    document.body.appendChild(input)
    dispatch({ key: 'k', metaKey: true, target: input })
    expect(handler).not.toHaveBeenCalled()

    const editable = document.createElement('div')
    editable.contentEditable = 'true'
    Object.defineProperty(editable, 'isContentEditable', {
      configurable: true,
      get: () => true,
    })
    document.body.appendChild(editable)
    dispatch({ key: 'k', metaKey: true, target: editable })
    expect(handler).not.toHaveBeenCalled()

    dispatch({ key: 'k', metaKey: true })
    expect(handler).toHaveBeenCalledTimes(1)

    document.body.removeChild(input)
    document.body.removeChild(editable)
  })

  it('lets the handler call preventDefault', () => {
    setPlatform('MacIntel')
    const handler = vi.fn((event: KeyboardEvent) => event.preventDefault())
    mount(Host, {
      props: { bindings: [{ key: 'k', modifiers: ['mod'], handler }] },
    })

    const event = dispatch({ key: 'k', metaKey: true })
    expect(handler).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })

  it('removes the listener on unmount', () => {
    setPlatform('MacIntel')
    const handler = vi.fn()
    const wrapper = mount(Host, {
      props: { bindings: [{ key: 'k', modifiers: ['mod'], handler }] },
    })

    dispatch({ key: 'k', metaKey: true })
    expect(handler).toHaveBeenCalledTimes(1)

    wrapper.unmount()

    dispatch({ key: 'k', metaKey: true })
    expect(handler).toHaveBeenCalledTimes(1)
  })
})
