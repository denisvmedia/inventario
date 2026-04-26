<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        variant="outline"
        role="combobox"
        :aria-expanded="open"
        aria-label="Currency"
        :disabled="loading"
        class="currency-select w-full justify-between font-normal"
      >
        <span class="truncate">
          {{ selectedLabel || (loading ? 'Loading currencies…' : 'Select a currency') }}
        </span>
        <ChevronsUpDown class="size-4 shrink-0 opacity-50" aria-hidden="true" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="p-0" align="start">
      <Command>
        <CommandInput placeholder="Type to search (e.g. USD, Euro)" />
        <CommandList>
          <CommandEmpty>No matching currency.</CommandEmpty>
          <CommandGroup>
            <CommandItem
              v-for="opt in currencies"
              :key="opt.code"
              :value="opt.label"
              @select="onSelect(opt.code)"
            >
              <Check
                class="size-4"
                :class="opt.code === modelValue ? 'opacity-100' : 'opacity-0'"
                aria-hidden="true"
              />
              {{ opt.label }}
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </Command>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Check, ChevronsUpDown } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@design/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@design/ui/popover'

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
const open = ref(false)

const selectedLabel = computed(() => {
  const match = currencies.value.find((c) => c.code === props.modelValue)
  return match?.label ?? ''
})

function onSelect(code: string) {
  emit('update:modelValue', code)
  open.value = false
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

    if (!props.modelValue) {
      emit('update:modelValue', props.defaultCurrency)
    }
  } catch (err) {
    console.error('Failed to load currencies', err)
  } finally {
    loading.value = false
  }
})
</script>
