import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import FileGallery from '../FileGallery.vue'

const confirmDelete = vi.hoisted(() => vi.fn())

vi.mock('@design/composables/useConfirm', () => ({
  useConfirm: () => ({ confirmDelete }),
}))

const imageFile = {
  id: 'file-1',
  attributes: {
    path: 'sample',
    ext: '.png',
    original_path: 'sample-original.png',
    mime_type: 'image/png',
  },
}

const pdfFile = {
  id: 'file-2',
  attributes: {
    path: 'manual',
    ext: '.pdf',
    original_path: 'manual.pdf',
    mime_type: 'application/pdf',
  },
}

function mountGallery(props: Record<string, unknown> = {}) {
  return mount(FileGallery, {
    props: {
      files: [imageFile, pdfFile],
      signedUrls: {
        'file-1': { url: '/sample.png', thumbnails: { medium: '/sample-thumb.png' } },
        'file-2': { url: '/manual.pdf' },
      },
      entityId: 'commodity-1',
      entityType: 'commodities',
      fileType: 'images',
      ...props,
    },
    attachTo: document.body,
  })
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

beforeEach(() => {
  confirmDelete.mockReset()
  confirmDelete.mockResolvedValue(true)
})

describe('FileGallery', () => {
  it('renders file previews with legacy anchors and signed thumbnails', () => {
    const wrapper = mountGallery()

    expect(wrapper.findAll('.file-item')).toHaveLength(2)
    expect(wrapper.get('.preview-image').attributes('src')).toBe('/sample-thumb.png')
    expect(wrapper.text()).toContain('sample.png')
    wrapper.unmount()
  })

  it('opens the file viewer dialog from a preview card', async () => {
    const wrapper = mountGallery()

    await wrapper.get('.file-card').trigger('click')
    await nextTick()

    expect(document.body.querySelector('.file-modal')).toBeTruthy()
    expect(document.body.textContent).toContain('sample.png')
    wrapper.unmount()
  })

  it('opens accessible details in a dialog primitive', async () => {
    const wrapper = mountGallery()

    await wrapper.get('[aria-label="View file details"]').trigger('click')
    await nextTick()

    const dialog = document.body.querySelector('[role="dialog"]')
    expect(dialog).toBeTruthy()
    expect(dialog?.textContent).toContain('File Details')
    expect(dialog?.textContent).toContain('sample-original.png')
    wrapper.unmount()
  })

  it('emits update when a file is renamed', async () => {
    const promptSpy = vi.spyOn(window, 'prompt').mockReturnValue('renamed')
    const wrapper = mountGallery()

    await wrapper.get('[aria-label="Edit file"]').trigger('click')

    expect(promptSpy).toHaveBeenCalledWith('Enter file name', 'sample')
    expect(wrapper.emitted('update')).toEqual([[{ id: 'file-1', path: 'renamed' }]])
    wrapper.unmount()
  })

  it('confirms deletion before emitting delete', async () => {
    const wrapper = mountGallery()

    await wrapper.get('[aria-label="Delete file"]').trigger('click')
    await flushPromises()

    expect(confirmDelete).toHaveBeenCalledWith('image')
    expect(wrapper.emitted('delete')).toEqual([[imageFile]])
    wrapper.unmount()
  })
})
