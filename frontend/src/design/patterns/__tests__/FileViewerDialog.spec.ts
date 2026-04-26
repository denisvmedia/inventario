import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { FileEntity } from '@/services/fileService'
import FileViewerDialog from '../FileViewerDialog.vue'

const PDFViewerCanvasStub = {
  name: 'PDFViewerCanvas',
  emits: ['error'],
  template: `
    <div class="pdf-stub">
      <button class="pdf-timeout" @click="$emit('error', { message: 'timeout while loading' })">Timeout</button>
      <button class="pdf-unknown" @click="$emit('error', { message: 'unknown failure' })">Unknown</button>
    </div>
  `,
}

vi.mock('@/components/PDFViewerCanvas.vue', () => ({
  __esModule: true,
  __isTeleport: false,
  __isSuspense: false,
  default: PDFViewerCanvasStub,
}))

function makeFile(overrides: Partial<FileEntity> = {}): FileEntity {
  return {
    id: 'image-1',
    title: 'Image',
    description: '',
    type: 'image',
    tags: [],
    path: 'sample',
    original_path: 'sample.png',
    ext: '.png',
    mime_type: 'image/png',
    ...overrides,
  }
}

const imageFile = makeFile()
const pdfFile = makeFile({
  id: 'pdf-1',
  title: 'Manual',
  type: 'document',
  path: 'manual',
  original_path: 'manual.pdf',
  ext: '.pdf',
  mime_type: 'application/pdf',
})
const secondPdfFile = makeFile({
  id: 'pdf-2',
  title: 'Invoice',
  type: 'document',
  path: 'invoice',
  original_path: 'invoice.pdf',
  ext: '.pdf',
  mime_type: 'application/pdf',
})

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(FileViewerDialog, {
    props: {
      open: true,
      files: [imageFile, pdfFile],
      selectedIndex: 0,
      signedUrls: {
        'image-1': { url: '/sample.png' },
        'pdf-1': { url: '/manual.pdf' },
        'pdf-2': { url: '/invoice.pdf' },
      },
      ...props,
    },
    attachTo: document.body,
  })
}

function clickBody(selector: string) {
  const element = document.body.querySelector<HTMLElement>(selector)
  expect(element).toBeTruthy()
  element!.click()
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('FileViewerDialog', () => {
  it('emits selected index updates from next/previous controls and keyboard navigation', async () => {
    const wrapper = mountDialog()
    await nextTick()

    clickBody('.nav-button.next')
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft' }))
    await nextTick()

    expect(wrapper.emitted('update:selectedIndex')).toEqual([[1], [1]])
    wrapper.unmount()
  })

  it('applies an actual scale transform when image zoom is enabled', async () => {
    const wrapper = mountDialog()
    await nextTick()

    clickBody('.full-image')
    await nextTick()

    const image = document.body.querySelector<HTMLImageElement>('.full-image')
    expect(image?.getAttribute('style')).toContain('scale(2)')
    wrapper.unmount()
  })

  it('renders dialog content above the overlay so embedded controls stay clickable', async () => {
    const wrapper = mountDialog({ selectedIndex: 1 })
    await flushPromises()

    const overlay = document.body.querySelector<HTMLElement>('[data-slot="dialog-overlay"]')
    const modal = document.body.querySelector<HTMLElement>('.file-modal')

    expect(overlay?.className).toContain('z-50')
    expect(modal?.className).toContain('z-[60]')
    wrapper.unmount()
  })

  it('resets stale PDF errors back to the default message for unknown failures', async () => {
    const wrapper = mountDialog({ selectedIndex: 1, files: [imageFile, pdfFile, secondPdfFile] })
    await flushPromises()

    clickBody('.pdf-timeout')
    await nextTick()
    expect(document.body.textContent).toContain('PDF loading timed out')

    await wrapper.setProps({ selectedIndex: 2 })
    await flushPromises()
    clickBody('.pdf-unknown')
    await nextTick()

    expect(document.body.textContent).toContain('Unable to display PDF')
    expect(document.body.textContent).not.toContain('PDF loading timed out')
    wrapper.unmount()
  })
})
