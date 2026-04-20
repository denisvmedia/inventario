<template>
  <Select
    :model-value="modelValue"
    :options="currencies"
    option-label="label"
    option-value="code"
    :placeholder="loading ? 'Loading currencies…' : 'Select a currency'"
    :disabled="loading"
    :filter="true"
    filter-placeholder="Type to search (e.g. USD, Euro)"
    :auto-filter-focus="true"
    class="currency-select w-100"
    @update:model-value="onUpdate"
  />
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Select from 'primevue/select'
import settingsService from '@/services/settingsService'

interface CurrencyOption {
  code: string
  label: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  defaultCurrency?: string
}>(), {
  defaultCurrency: 'USD',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const currencies = ref<CurrencyOption[]>([])
const loading = ref(true)

function onUpdate(value: string) {
  emit('update:modelValue', value)
}

onMounted(async () => {
  try {
    const response = await settingsService.getCurrencies()
    const codes: string[] = response.data || []
    // Intl.DisplayNames gives the localized currency name (e.g. "US Dollar").
    // Failing gracefully per-code is important: bojanz/currency includes a
    // handful of historical/obscure ISO codes that Intl may not recognize —
    // we keep them in the list with just the code as the label rather than
    // dropping them entirely.
    const names = new Intl.DisplayNames(['en'], { type: 'currency' })
    currencies.value = codes
      .map((code) => {
        let label = code
        try {
          const name = names.of(code)
          if (name && name !== code) {
            label = `${code} — ${name}`
          }
        } catch { /* fall through: use bare code */ }
        return { code, label }
      })
      .sort((a, b) => a.code.localeCompare(b.code))

    // Seed the default so the caller never submits an empty main_currency
    // when the user accepts the form as-is. Only apply when the caller hasn't
    // already set a value (e.g. editing an existing group in the future).
    if (!props.modelValue) {
      emit('update:modelValue', props.defaultCurrency)
    }
  } catch (err) {
    // A failed fetch leaves the dropdown empty; the caller sees a disabled
    // control and can retry. We don't want to silently fall back to a
    // hardcoded list here — that would re-introduce the very regression this
    // component exists to fix.
    console.error('Failed to load currencies', err)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped lang="scss">
.currency-select {
  width: 100%;
}
</style>
