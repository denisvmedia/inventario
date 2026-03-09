import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SystemSettingDetailView from '../SystemSettingDetailView.vue'

const { mockRouter, mockRoute, mockSettingsService, mockSettingsStore } = vi.hoisted(() => ({
  mockRouter: {
    push: vi.fn()
  },
  mockRoute: {
    params: { id: 'system_config' },
    query: {}
  },
  mockSettingsService: {
    getSettings: vi.fn(),
    getCurrencies: vi.fn()
  },
  mockSettingsStore: {
    updateMainCurrency: vi.fn(),
    error: null as string | null
  }
}))

vi.mock('vue-router', () => ({
  useRoute: () => mockRoute,
  useRouter: () => mockRouter
}))

vi.mock('@/services/settingsService', () => ({
  default: mockSettingsService
}))

vi.mock('@/stores/settingsStore', () => ({
  useSettingsStore: () => mockSettingsStore
}))

const SelectStub = {
  props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'disabled'],
  emits: ['update:modelValue'],
  template: `
    <select
      :id="$attrs.id"
      :disabled="disabled"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option
        v-for="option in options"
        :key="option[optionValue]"
        :value="option[optionValue]"
      >
        {{ option[optionLabel] }}
      </option>
    </select>
  `
}

const flushPromises = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

const mountView = async () => {
  const wrapper = mount(SystemSettingDetailView, {
    global: {
      stubs: {
        Select: SelectStub,
        RouterLink: true,
        'router-link': true,
        'font-awesome-icon': true
      }
    }
  })

  await flushPromises()
  return wrapper
}

describe('SystemSettingDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    mockSettingsStore.error = null
    mockSettingsStore.updateMainCurrency.mockResolvedValue(undefined)
    mockSettingsService.getSettings.mockResolvedValue({
      data: {
        MainCurrency: 'USD'
      }
    })
    mockSettingsService.getCurrencies.mockResolvedValue({
      data: ['USD', 'EUR']
    })
  })

  it('allows changing an existing main currency and sending an optional exchange rate', async () => {
    const wrapper = await mountView()

    expect(wrapper.find('#main-currency').attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('cannot be changed once set')

    await wrapper.find('#main-currency').setValue('EUR')
    await flushPromises()
    await wrapper.find('#exchange-rate').setValue('0.95')
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mockSettingsStore.updateMainCurrency).toHaveBeenCalledWith('EUR', '0.95')
    expect(mockRouter.push).toHaveBeenCalledWith({
      path: '/system',
      query: { success: 'true' }
    })
  })

  it('submits the currency change without an exchange rate when left blank', async () => {
    const wrapper = await mountView()

    await wrapper.find('#main-currency').setValue('EUR')
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mockSettingsStore.updateMainCurrency).toHaveBeenCalledWith('EUR', undefined)
  })

  it('shows exchange-rate backend errors inline', async () => {
    mockSettingsStore.error = 'Exchange rate must be greater than zero'
    mockSettingsStore.updateMainCurrency.mockRejectedValue(new Error('bad exchange rate'))

    const wrapper = await mountView()

    await wrapper.find('#main-currency').setValue('EUR')
    await flushPromises()
    await wrapper.find('#exchange-rate').setValue('1.1')
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Exchange rate must be greater than zero')
    expect(mockRouter.push).not.toHaveBeenCalled()
  })
})