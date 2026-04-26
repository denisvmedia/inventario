import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ResourceNotFound from '../ResourceNotFound.vue'

function findButton(wrapper: ReturnType<typeof mount>, label: string) {
  return wrapper.findAll('button').find((btn) => btn.text().includes(label))
}

describe('ResourceNotFound.vue', () => {
  it('renders with default props', () => {
    const wrapper = mount(ResourceNotFound)

    expect(wrapper.find('h3').text()).toBe('Error Loading Resource')
    expect(wrapper.find('p').text()).toBe('The resource was not found. It may have been deleted or moved.')
    expect(findButton(wrapper, 'Go Back')).toBeDefined()
    expect(findButton(wrapper, 'Try Again')).toBeDefined()
  })

  it('renders with custom resource type', () => {
    const wrapper = mount(ResourceNotFound, {
      props: {
        resourceType: 'commodity'
      }
    })

    expect(wrapper.find('h3').text()).toBe('Error Loading Commodity')
    expect(wrapper.find('p').text()).toBe('The commodity was not found. It may have been deleted or moved.')
  })

  it('renders with custom title and message', () => {
    const wrapper = mount(ResourceNotFound, {
      props: {
        title: 'Custom Title',
        message: 'Custom message here'
      }
    })

    expect(wrapper.find('h3').text()).toBe('Custom Title')
    expect(wrapper.find('p').text()).toBe('Custom message here')
  })

  it('renders with custom button text', () => {
    const wrapper = mount(ResourceNotFound, {
      props: {
        goBackText: 'Back to List',
        tryAgainText: 'Reload'
      }
    })

    expect(findButton(wrapper, 'Back to List')).toBeDefined()
    expect(findButton(wrapper, 'Reload')).toBeDefined()
  })

  it('hides buttons when configured', () => {
    const wrapper = mount(ResourceNotFound, {
      props: {
        showGoBack: false,
        showTryAgain: false
      }
    })

    expect(findButton(wrapper, 'Go Back')).toBeUndefined()
    expect(findButton(wrapper, 'Try Again')).toBeUndefined()
  })

  it('emits go-back event when Go Back button is clicked', async () => {
    const wrapper = mount(ResourceNotFound)
    const goBack = findButton(wrapper, 'Go Back')!
    await goBack.trigger('click')

    expect(wrapper.emitted('go-back')).toBeTruthy()
    expect(wrapper.emitted('go-back')).toHaveLength(1)
  })

  it('emits try-again event when Try Again button is clicked', async () => {
    const wrapper = mount(ResourceNotFound)
    const tryAgain = findButton(wrapper, 'Try Again')!
    await tryAgain.trigger('click')

    expect(wrapper.emitted('try-again')).toBeTruthy()
    expect(wrapper.emitted('try-again')).toHaveLength(1)
  })

  it('renders custom actions slot', () => {
    const wrapper = mount(ResourceNotFound, {
      slots: {
        'custom-actions': '<button class="custom-btn">Custom Action</button>'
      }
    })

    expect(wrapper.find('.custom-btn').exists()).toBe(true)
    expect(wrapper.find('.custom-btn').text()).toBe('Custom Action')
  })

  it('applies correct CSS classes', () => {
    const wrapper = mount(ResourceNotFound)

    expect(wrapper.find('.resource-not-found').exists()).toBe(true)
    expect(wrapper.find('.error-icon').exists()).toBe(true)
    expect(wrapper.find('.error-actions').exists()).toBe(true)
  })
})
