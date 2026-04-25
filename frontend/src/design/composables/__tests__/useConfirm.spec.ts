import { beforeEach, describe, expect, it, vi } from 'vitest'

const showMock = vi.fn<(_opts?: Record<string, unknown>) => Promise<boolean>>()

vi.mock('@/stores/confirmationStore', () => ({
  useConfirmationStore: () => ({ show: showMock }),
}))

import { useConfirm } from '../useConfirm'

describe('useConfirm', () => {
  beforeEach(() => {
    showMock.mockReset()
  })

  it('forwards confirm() options unchanged to confirmationStore.show', async () => {
    showMock.mockResolvedValueOnce(true)
    const { confirm } = useConfirm()

    const result = await confirm({
      title: 'Leave page?',
      message: 'You have unsaved changes.',
      confirmLabel: 'Leave',
      cancelLabel: 'Stay',
      confirmButtonClass: 'warning',
    })

    expect(result).toBe(true)
    expect(showMock).toHaveBeenCalledWith({
      title: 'Leave page?',
      message: 'You have unsaved changes.',
      confirmLabel: 'Leave',
      cancelLabel: 'Stay',
      confirmButtonClass: 'warning',
    })
  })

  it('resolves to false when the user cancels', async () => {
    showMock.mockResolvedValueOnce(false)
    const { confirm } = useConfirm()

    await expect(confirm()).resolves.toBe(false)
    expect(showMock).toHaveBeenCalledWith({})
  })

  it('confirmDelete(itemType) fills destructive defaults', async () => {
    showMock.mockResolvedValueOnce(true)
    const { confirmDelete } = useConfirm()

    await confirmDelete('area')

    expect(showMock).toHaveBeenCalledWith({
      title: 'Confirm Delete',
      message: 'Are you sure you want to delete this area?',
      confirmLabel: 'Delete',
      cancelLabel: 'Cancel',
      confirmButtonClass: 'danger',
    })
  })

  it('confirmDelete honours caller overrides', async () => {
    showMock.mockResolvedValueOnce(true)
    const { confirmDelete } = useConfirm()

    await confirmDelete('commodity', {
      title: 'Destroy commodity?',
      confirmLabel: 'Destroy',
    })

    expect(showMock).toHaveBeenCalledWith({
      title: 'Destroy commodity?',
      message: 'Are you sure you want to delete this commodity?',
      confirmLabel: 'Destroy',
      cancelLabel: 'Cancel',
      confirmButtonClass: 'danger',
    })
  })
})
