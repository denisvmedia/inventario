import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import type { FileEntity } from '@/services/fileService'
import FilePreview from '../FilePreview.vue'

const RouterLinkStub = {
  name: 'RouterLink',
  props: ['to'],
  template: '<a :href="typeof to === \'string\' ? to : to?.path"><slot /></a>',
}

function makeFile(over: Partial<FileEntity> = {}): FileEntity {
  return {
    id: 'f-1',
    title: 'My Receipt',
    description: 'Coffee receipt',
    type: 'image',
    tags: [],
    path: 'receipts/coffee',
    original_path: 'coffee.png',
    ext: '.png',
    mime_type: 'image/png',
    ...over,
  }
}

function mountFP(props: Record<string, unknown>) {
  return mount(FilePreview, {
    props,
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

describe('FilePreview', () => {
  it('renders the title from file.title', () => {
    const wrapper = mountFP({ file: makeFile({ title: 'Statement.pdf' }) })

    expect(wrapper.get('h3').text()).toBe('Statement.pdf')
  })

  it('falls back to file.path when title is empty', () => {
    const wrapper = mountFP({
      file: makeFile({ title: '', path: 'receipts/coffee' }),
    })

    expect(wrapper.get('h3').text()).toBe('receipts/coffee')
  })

  it('falls back to "Untitled" when both title and path are empty', () => {
    const wrapper = mountFP({ file: makeFile({ title: '', path: '' }) })

    expect(wrapper.get('h3').text()).toBe('Untitled')
  })

  it('renders the description or "No description" when blank', () => {
    const withDesc = mountFP({ file: makeFile({ description: 'Hello' }) })
    expect(withDesc.text()).toContain('Hello')

    const blank = mountFP({ file: makeFile({ description: '' }) })
    expect(blank.text()).toContain('No description')
  })

  it('renders the thumbnail image when file is an image and thumbnailUrl is provided', () => {
    const wrapper = mountFP({
      file: makeFile(),
      thumbnailUrl: '/thumb.png',
    })

    const img = wrapper.get('img')
    expect(img.attributes('src')).toBe('/thumb.png')
    expect(img.attributes('alt')).toBe('My Receipt')
  })

  it('renders an icon when file is not an image', () => {
    const wrapper = mountFP({
      file: makeFile({ type: 'document', mime_type: 'application/pdf' }),
    })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.findAll('svg').length).toBeGreaterThan(0)
  })

  it('renders an icon when file is image but no thumbnail URL is supplied', () => {
    const wrapper = mountFP({ file: makeFile() })

    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('renders the file type label and extension chips', () => {
    const wrapper = mountFP({
      file: makeFile({ type: 'document', ext: '.pdf' }),
    })

    expect(wrapper.text()).toContain('Document')
    expect(wrapper.text()).toContain('.pdf')
  })

  it('renders the first three tags + overflow counter', () => {
    const wrapper = mountFP({
      file: makeFile({ tags: ['a', 'b', 'c', 'd', 'e'] }),
    })

    expect(wrapper.text()).toContain('a')
    expect(wrapper.text()).toContain('b')
    expect(wrapper.text()).toContain('c')
    expect(wrapper.text()).toContain('+2 more')
  })

  it('omits the overflow counter when there are three or fewer tags', () => {
    const wrapper = mountFP({ file: makeFile({ tags: ['a', 'b'] }) })

    expect(wrapper.text()).not.toContain('more')
  })

  it('renders a linked-entity router-link when linkedEntity is provided', () => {
    const wrapper = mountFP({
      file: makeFile(),
      linkedEntity: {
        display: 'Office',
        url: '/g/x/locations/abc',
        icon: 'location',
      },
    })

    const link = wrapper.get('a')
    expect(link.attributes('href')).toBe('/g/x/locations/abc')
    expect(link.text()).toContain('Office')
  })

  it('keeps the .file-card legacy anchor on the root', () => {
    const wrapper = mountFP({ file: makeFile() })

    expect(wrapper.classes()).toContain('file-card')
  })

  it('forwards testId and data-file-id to the root', () => {
    const wrapper = mountFP({
      file: makeFile({ id: 'abc' }),
      testId: 'preview-1',
    })

    expect(wrapper.attributes('data-testid')).toBe('preview-1')
    expect(wrapper.attributes('data-file-id')).toBe('abc')
  })

  it('emits view on click and Enter / Space', async () => {
    const wrapper = mountFP({ file: makeFile() })

    await wrapper.trigger('click')
    await wrapper.trigger('keydown', { key: 'Enter' })
    await wrapper.trigger('keydown', { key: ' ' })

    expect(wrapper.emitted('view')).toHaveLength(3)
  })

  it('emits download / edit / delete from the action buttons without bubbling view', async () => {
    const wrapper = mountFP({ file: makeFile() })

    await wrapper.get('[aria-label="Download file"]').trigger('click')
    await wrapper.get('[aria-label="Edit file"]').trigger('click')
    await wrapper.get('[aria-label="Delete file"]').trigger('click')

    expect(wrapper.emitted('download')).toHaveLength(1)
    expect(wrapper.emitted('edit')).toHaveLength(1)
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('view')).toBeUndefined()
  })

  it('replaces the trash button with a disabled lock when canDelete is false', () => {
    const wrapper = mountFP({
      file: makeFile(),
      canDelete: false,
      deleteRestrictionReason: 'Export files cannot be deleted manually.',
    })

    expect(wrapper.find('[aria-label="Delete file"]').exists()).toBe(false)
    const lock = wrapper.get(
      '[aria-label="Export files cannot be deleted manually."]',
    )
    expect(lock.attributes('disabled')).toBeDefined()
  })

  it('emits imageError when the thumbnail fails to load', async () => {
    const wrapper = mountFP({
      file: makeFile(),
      thumbnailUrl: '/broken.png',
    })

    await wrapper.get('img').trigger('error')

    expect(wrapper.emitted('imageError')).toHaveLength(1)
  })
})
