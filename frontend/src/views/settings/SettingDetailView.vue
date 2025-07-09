<template>
  <div class="setting-detail">
    <div class="header">
      <div class="back-link">
        <router-link to="/settings">
          <font-awesome-icon icon="arrow-left" /> Back to Settings
        </router-link>
      </div>
      <h1>{{ settingTitle }}</h1>
    </div>

    <div v-if="loading" class="loading">
      Loading setting...
    </div>
    <div v-else-if="error" class="error">
      {{ error }}
    </div>

    <!-- Settings Required Banner for System Settings -->
    <div v-if="settingId === 'system_config' && isSettingsRequired && settingsLoaded" class="settings-required-banner">
      <div class="banner-icon">
        <font-awesome-icon icon="exclamation-triangle" />
      </div>
      <div class="banner-content">
        <p><strong>Settings Required:</strong> Please configure the Main Currency to continue using the application.</p>
      </div>
    </div>
    <!-- UI Settings -->
    <div v-else-if="settingId === 'ui_config'" class="form">
      <div class="form-group">
        <label for="theme">Theme</label>
        <Select
          id="theme"
          v-model="uiConfig.theme"
          :options="themeOptions"
          option-label="name"
          option-value="id"
          placeholder="Select a theme"
          class="w-100"
          :class="{ 'is-invalid': formErrors.theme }"
          aria-label="Theme"
        />
        <div v-if="formErrors.theme" class="error-message">{{ formErrors.theme }}</div>
      </div>

      <div class="form-group">
        <label for="show-debug">Show Debug Information</label>
        <input
          id="show-debug"
          v-model="uiConfig.show_debug_info"
          type="checkbox"
        />
      </div>

      <div class="form-group">
        <label for="page-size">Default Page Size</label>
        <input
          id="page-size"
          v-model="uiConfig.default_page_size"
          type="number"
          class="form-control"
          min="5"
          max="100"
        />
      </div>

      <div class="form-group">
        <label for="date-format">Date Format</label>
        <Select
          id="date-format"
          v-model="uiConfig.default_date_format"
          :options="dateFormatOptions"
          option-label="name"
          option-value="id"
          placeholder="Select a date format"
          class="w-100"
          :class="{ 'is-invalid': formErrors.default_date_format }"
          aria-label="Date Format"
        />
        <div v-if="formErrors.default_date_format" class="error-message">{{ formErrors.default_date_format }}</div>
      </div>

      <div class="form-actions">
        <button class="btn btn-secondary" @click="goBack">Cancel</button>
        <button
          class="btn btn-primary"
          :disabled="isSubmitting"
          @click="saveUIConfig"
        >
          {{ isSubmitting ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>

    <!-- System Settings -->
    <div v-else-if="settingId === 'system_config'" class="form">
      <div class="form-group">
        <label for="main-currency">Main Currency</label>
        <Select
          id="main-currency"
          v-model="systemConfig.main_currency"
          :options="currencies"
          option-label="label"
          option-value="id"
          placeholder="Select a currency"
          class="w-100"
          :class="{ 'is-invalid': formErrors.main_currency }"
          :filter="true"
          :show-clear="false"
          aria-label="Currency"
          :disabled="isMainCurrencySet"
        />
        <div v-if="formErrors.main_currency" class="error-message">{{ formErrors.main_currency }}</div>
        <div v-if="isMainCurrencySet" class="currency-locked-message">
          <font-awesome-icon icon="lock" /> Main currency is locked and cannot be changed once set.
        </div>
      </div>

      <div class="form-actions">
        <button class="btn btn-secondary" @click="goBack">Cancel</button>
        <button
          class="btn btn-primary"
          :disabled="isSubmitting || (isMainCurrencySet && systemConfig.main_currency === originalMainCurrency)"
          @click="saveSystemConfig"
        >
          {{ isSubmitting ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>

    <!-- Custom Settings (JSON Editor) -->
    <div v-else class="form">
      <div class="form-group">
        <label for="json-editor">JSON Value</label>
        <textarea
          id="json-editor"
          v-model="jsonValue"
          class="form-control json-editor"
          :class="{ 'is-invalid': jsonError }"
          rows="10"
        ></textarea>
        <div v-if="jsonError" class="error-message">{{ jsonError }}</div>
      </div>

      <div class="form-actions">
        <button class="btn btn-secondary" @click="goBack">Cancel</button>
        <button
          class="btn btn-primary"
          :disabled="isSubmitting || jsonError"
          @click="saveCustomSetting"
        >
          {{ isSubmitting ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import settingsService from '@/services/settingsService'
import { useSettingsStore } from '@/stores/settingsStore'

import Select from 'primevue/select'

const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const settingId = computed(() => route.params.id as string)

const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const jsonValue = ref<string>('')
const jsonError = ref<string | null>(null)

const currencies = ref<any[]>([])

// Theme options
const themeOptions = ref([
  { id: 'light', name: 'Light' },
  { id: 'dark', name: 'Dark' },
  { id: 'system', name: 'System' }
])

// Date format options
const dateFormatOptions = ref([
  { id: 'YYYY-MM-DD', name: 'YYYY-MM-DD' },
  { id: 'MM/DD/YYYY', name: 'MM/DD/YYYY' },
  { id: 'DD/MM/YYYY', name: 'DD/MM/YYYY' },
  { id: 'DD.MM.YYYY', name: 'DD.MM.YYYY' }
])

// Track if we've loaded settings
const settingsLoaded = ref(false)

// Check if settings are required based on actual state, not just URL parameter
const isSettingsRequired = computed(() => {
  // Only check URL parameter if settings haven't been loaded yet
  if (!settingsLoaded.value) {
    return route.query.required === 'true'
  }
  // After settings are loaded, check actual state: settings are required only if MainCurrency is not set
  return settingId.value === 'system_config' && !isMainCurrencySet.value
})

// Removed Currency Config and TLS Config as requested

// UI Config
const uiConfig = ref({
  theme: 'light',
  show_debug_info: false,
  default_page_size: 20,
  default_date_format: 'YYYY-MM-DD'
})

// System Config
const systemConfig = ref({
  upload_size_limit: 10485760, // 10MB
  log_level: 'info',
  backup_enabled: false,
  backup_interval: '24h',
  backup_location: '',
  main_currency: 'USD'
})

// Track if main currency is already set
const isMainCurrencySet = ref<boolean>(false)
const originalMainCurrency = ref<string>('')

// Form validation errors
const formErrors = ref({
  theme: '',
  default_date_format: '',
  main_currency: ''
})

const settingTitle = computed(() => {
  switch (settingId.value) {
    case 'ui_config':
      return 'User Interface Settings'
    case 'system_config':
      return 'System Settings'
    default:
      return formatSettingName(settingId.value)
  }
})

// Watch for changes in the JSON editor
watch(jsonValue, (newValue) => {
  try {
    if (newValue.trim()) {
      JSON.parse(newValue)
      jsonError.value = null
    } else {
      jsonError.value = null
    }
  } catch (err: any) {
    jsonError.value = 'Invalid JSON: ' + err.message
  }
})

// Fetch currencies for the dropdown
const fetchCurrencies = async () => {
  try {
    const response = await settingsService.getCurrencies()

    // Use Intl.DisplayNames to get currency names
    const currencyNames = new Intl.DisplayNames(['en'], { type: 'currency' })

    // Transform the array of currency codes into the expected format with names
    currencies.value = response.data.map((code: string) => {
      let currencyName = code
      try {
        // Try to get the localized currency name
        currencyName = currencyNames.of(code)
      } catch {
        console.warn(`Could not get display name for currency: ${code}`)
      }

      return {
        id: code,
        label: `${currencyName} (${code})`,
        value: code
      }
    })
  } catch (err) {
    console.error('Error fetching currencies:', err)
  }
}

onMounted(async () => {
  await loadSetting()
  await fetchCurrencies()
  // Mark settings as loaded to prevent flashing the banner
  settingsLoaded.value = true
})

async function loadSetting() {
  loading.value = true
  error.value = null

  try {
    const response = await settingsService.getSettings()
    const settings = response.data

    if (settingId.value === 'ui_config') {
      // Set UI config
      uiConfig.value = {
        theme: settings.Theme || 'light',
        show_debug_info: settings.ShowDebugInfo || false,
        default_page_size: 20, // This is not in the settings model, using default
        default_date_format: settings.DefaultDateFormat || 'YYYY-MM-DD'
      }
    } else if (settingId.value === 'system_config') {
      // Set System config
      systemConfig.value = {
        upload_size_limit: 10485760, // Not in settings model, using default
        log_level: 'info', // Not in settings model, using default
        backup_enabled: false, // Not in settings model, using default
        backup_interval: '24h', // Not in settings model, using default
        backup_location: '', // Not in settings model, using default
        main_currency: settings.MainCurrency || 'USD'
      }

      // Check if main currency is already set
      if (settings.MainCurrency && settings.MainCurrency !== '') {
        isMainCurrencySet.value = true
        originalMainCurrency.value = settings.MainCurrency
      } else {
        isMainCurrencySet.value = false
      }
    } else {
      // For custom settings, just show empty JSON
      jsonValue.value = '{}'
    }
  } catch (err: any) {
    // Handle error
    error.value = 'Failed to load settings: ' + (err.message || 'Unknown error')
    console.error('Error loading settings:', err)

    // Use defaults
    if (settingId.value === 'ui_config') {
      uiConfig.value = {
        theme: 'light',
        show_debug_info: false,
        default_page_size: 20,
        default_date_format: 'YYYY-MM-DD'
      }
    } else if (settingId.value === 'system_config') {
      systemConfig.value = {
        upload_size_limit: 10485760,
        log_level: 'info',
        backup_enabled: false,
        backup_interval: '24h',
        backup_location: '',
        main_currency: 'USD'
      }
    } else {
      jsonValue.value = '{}'
    }
  } finally {
    loading.value = false
    // Mark settings as loaded to properly handle the banner visibility
    settingsLoaded.value = true
  }
}

function goBack() {
  router.push('/settings')
}

// Removed saveCurrencyConfig and saveTLSConfig functions as requested

async function saveUIConfig() {
  // Reset validation errors
  formErrors.value.theme = ''
  formErrors.value.default_date_format = ''

  // Validate
  let isValid = true

  if (!uiConfig.value.theme) {
    formErrors.value.theme = 'Theme is required'
    isValid = false
  }

  if (!uiConfig.value.default_date_format) {
    formErrors.value.default_date_format = 'Date Format is required'
    isValid = false
  }

  if (!isValid) {
    return
  }

  isSubmitting.value = true
  try {
    // Update theme
    await settingsService.updateTheme(uiConfig.value.theme)

    // Update show debug info
    await settingsService.updateShowDebugInfo(uiConfig.value.show_debug_info)

    // Update default date format
    await settingsService.updateDefaultDateFormat(uiConfig.value.default_date_format)

    // Note: default_page_size is not in the settings model, so we don't update it

    router.push('/settings')
  } catch (err: any) {
    error.value = 'Failed to save UI config: ' + (err.message || 'Unknown error')
    console.error('Error saving UI config:', err)
  } finally {
    isSubmitting.value = false
  }
}

async function saveSystemConfig() {
  // Reset validation errors
  formErrors.value.main_currency = ''

  // Validate
  let isValid = true

  if (!systemConfig.value.main_currency) {
    formErrors.value.main_currency = 'Main Currency is required'
    isValid = false
  }

  if (!isValid) {
    return
  }

  // If main currency is already set and hasn't changed, just go back to settings
  if (isMainCurrencySet.value && systemConfig.value.main_currency === originalMainCurrency.value) {
    router.push('/settings')
    return
  }

  // If main currency is already set and has changed, show error and prevent saving
  if (isMainCurrencySet.value && systemConfig.value.main_currency !== originalMainCurrency.value) {
    formErrors.value.main_currency = 'Main currency cannot be changed once it has been set'
    return
  }

  isSubmitting.value = true
  try {
    // Update main currency using the store
    await settingsStore.updateMainCurrency(systemConfig.value.main_currency)

    // Note: other system config fields are not in the settings model, so we don't update them

    // Redirect to settings with success message
    router.push({
      path: '/settings',
      query: { success: 'true' }
    })
  } catch (err: any) {
    // Check if this is the specific error about main currency already being set
    if (err.response && err.response.status === 422 &&
        err.response.data && err.response.data.includes('main currency already set')) {
      formErrors.value.main_currency = 'Main currency has already been set and cannot be changed'
      // Update UI to reflect that main currency is now set
      isMainCurrencySet.value = true
      originalMainCurrency.value = systemConfig.value.main_currency
    } else {
      error.value = 'Failed to save System config: ' + (err.message || 'Unknown error')
    }
    console.error('Error saving System config:', err)
  } finally {
    isSubmitting.value = false
  }
}

async function saveCustomSetting() {
  if (jsonError.value) return

  isSubmitting.value = true
  try {
    // Custom settings are not supported in the new API
    // This is a placeholder for future implementation
    error.value = 'Custom settings are not supported in the current version'
    console.warn('Custom settings are not supported in the current version')
    isSubmitting.value = false
  } catch (err: any) {
    error.value = 'Failed to save setting: ' + (err.message || 'Unknown error')
    console.error('Error saving setting:', err)
    isSubmitting.value = false
  }
}

function formatSettingName(id: string) {
  // Convert snake_case or kebab-case to Title Case
  return id
    .replace(/[-_]/g, ' ')
    .replace(/\w\S*/g, (txt) => txt.charAt(0).toUpperCase() + txt.substr(1).toLowerCase())
}


</script>

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.setting-detail {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;

  .header {
    margin-bottom: 20px;

    .back-link {
      margin-bottom: 10px;

      a {
        color: $secondary-color;
        text-decoration: none;
        display: inline-flex;
        align-items: center;
        gap: 5px;

        &:hover {
          text-decoration: underline;
        }
      }
    }

    h1 {
      margin: 0;
      color: $primary-color;
    }
  }

  .settings-required-banner {
    display: flex;
    background-color: #fff3cd;
    border: 1px solid #ffeeba;
    border-radius: 8px;
    padding: 15px;
    margin-bottom: 20px;
    box-shadow: $box-shadow;

    .banner-icon {
      color: #856404;
      margin-right: 15px;
      display: flex;
      align-items: center;
    }

    .banner-content {
      flex: 1;

      p {
        margin: 0;
        color: #856404;
      }
    }
  }

  .loading, .error {
    text-align: center;
    padding: 20px;
    background-color: #f9f9f9;
    border-radius: 8px;
    margin-top: 20px;
  }

  .error {
    color: $error-color;
  }
}

/* Ensure consistent styling between settings and commodity forms */
.setting-detail .p-dropdown,
.setting-detail .p-select {
  font-size: 1rem;
  padding: 0;
  border: 1px solid #ddd;
  border-radius: 4px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.setting-detail .p-dropdown:not(.p-disabled):hover,
.setting-detail .p-select:not(.p-disabled):hover {
  border-color: #bbb;
}

.setting-detail .p-dropdown:not(.p-disabled).p-focus,
.setting-detail .p-select:not(.p-disabled).p-focus {
  border-color: #4CAF50;
  box-shadow: 0 0 0 2px rgb(76 175 80 / 20%);
  outline: none;
}

/* Match the height of the dropdown to the form-control height */
.setting-detail .p-dropdown .p-dropdown-label,
.setting-detail .p-select .p-select-label {
  padding: 0.75rem;
  line-height: 1.5;
}

/* Ensure the dropdown trigger icon is properly aligned */
.setting-detail .p-dropdown .p-dropdown-trigger,
.setting-detail .p-select .p-select-trigger {
  padding: 0 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

// Error message styling to match CommodityForm.vue
.error-message {
  color: $danger-color;
  font-size: 0.875rem;
  margin-top: 0.25rem;
}

// Currency locked message styling
.currency-locked-message {
  color: $secondary-color;
  font-size: 0.875rem;
  margin-top: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-style: italic;
}

@media (width <= 768px) {
  .setting-detail {
    .form-actions {
      flex-direction: column;

      button {
        width: 100%;
      }
    }
  }
}
</style>
