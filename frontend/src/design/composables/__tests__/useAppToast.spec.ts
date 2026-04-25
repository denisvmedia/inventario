import { beforeEach, describe, expect, it, vi } from 'vitest'

// Mock vue-sonner before importing the composable — the module-level
// destructure in useAppToast captures the mocked `toast` object.
vi.mock('vue-sonner', () => {
  const toast = {
    success: vi.fn(() => 1),
    error: vi.fn(() => 2),
    warning: vi.fn(() => 3),
    info: vi.fn(() => 4),
    dismiss: vi.fn(),
  }
  return { toast }
})

import { toast as sonnerToast } from 'vue-sonner'
import { useAppToast } from '../useAppToast'

describe('useAppToast', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('forwards success(title, opts) to sonner.success', () => {
    const t = useAppToast()
    const opts = { description: 'Saved to disk' }

    t.success('Saved', opts)

    expect(sonnerToast.success).toHaveBeenCalledTimes(1)
    expect(sonnerToast.success).toHaveBeenCalledWith('Saved', opts)
  })

  it('forwards warning() and info() to the matching sonner methods', () => {
    const t = useAppToast()

    t.warning('Careful')
    t.info('FYI', { duration: 3000 })

    expect(sonnerToast.warning).toHaveBeenCalledWith('Careful', undefined)
    expect(sonnerToast.info).toHaveBeenCalledWith('FYI', { duration: 3000 })
  })

  it('accepts a string message on error() and passes it through', () => {
    const t = useAppToast()

    t.error('Could not save', { duration: 5000 })

    expect(sonnerToast.error).toHaveBeenCalledTimes(1)
    expect(sonnerToast.error).toHaveBeenCalledWith('Could not save', { duration: 5000 })
  })

  it('unwraps an Error instance on error() using its message', () => {
    const t = useAppToast()
    const err = new Error('Network down')

    t.error(err)

    expect(sonnerToast.error).toHaveBeenCalledTimes(1)
    expect(sonnerToast.error).toHaveBeenCalledWith('Network down', undefined)
  })

  it('dismiss(id) forwards to sonner.dismiss', () => {
    const t = useAppToast()

    t.dismiss('abc')
    t.dismiss()

    expect(sonnerToast.dismiss).toHaveBeenNthCalledWith(1, 'abc')
    expect(sonnerToast.dismiss).toHaveBeenNthCalledWith(2, undefined)
  })
})
