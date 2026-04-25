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
    :show-clear="false"
    aria-label="Currency"
    class="currency-select w-100"
    @update:model-value="onUpdate"
  />
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1328
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

function formatLabel(code: string, names: Intl.DisplayNames): string {
  let name: string | undefined
  try {
    name = names.of(code)
  } catch { /* leave name undefined */ }

  let symbol: string | undefined
  try {
    const parts = new Intl.NumberFormat('en', {
      style: 'currency',
      currency: code,
      currencyDisplay: 'narrowSymbol',
    }).formatToParts(0)
    const part = parts.find((p) => p.type === 'currency')
    if (part && part.value && part.value !== code) {
      symbol = part.value
    }
  } catch { /* leave symbol undefined */ }

  if (!name || name === code) {
    // Intl couldn't localize — fall back to the bare code so the entry
    // stays useful instead of appearing as a duplicate in the list.
    return code
  }
  return symbol ? `${code} — ${name} (${symbol})` : `${code} — ${name}`
}

onMounted(async () => {
  try {
    const response = await settingsService.getCurrencies()
    const codes: string[] = response.data || []
    // Labels follow the format spelled out in issue #1256's acceptance
    // criteria: `CODE — Name (symbol)`, e.g. `EUR — Euro (€)`. The symbol
    // falls out of Intl.NumberFormat's parts; when it matches the code
    // verbatim (common for minor / historical currencies) we omit the
    // parenthesised segment rather than print `USD — US Dollar (USD)`.
    // Each lookup is guarded because bojanz/currency lists a handful of
    // codes Intl doesn't recognise — we keep them in the dropdown with
    // a bare-code label so they remain selectable.
    const names = new Intl.DisplayNames(['en'], { type: 'currency' })
    currencies.value = codes
      .map((code) => ({ code, label: formatLabel(code, names) }))
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
