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
        <select id="theme" class="form-control" v-model="uiConfig.theme">
          <option value="light">Light</option>
          <option value="dark">Dark</option>
          <option value="system">System</option>
        </select>
      </div>

      <div class="form-group">
        <label for="show-debug">Show Debug Information</label>
        <input
          type="checkbox"
          id="show-debug"
          v-model="uiConfig.show_debug_info"
        />
      </div>

      <div class="form-group">
        <label for="page-size">Default Page Size</label>
        <input
          type="number"
          id="page-size"
          class="form-control"
          v-model="uiConfig.default_page_size"
          min="5"
          max="100"
        />
      </div>

      <div class="form-group">
        <label for="date-format">Date Format</label>
        <select id="date-format" class="form-control" v-model="uiConfig.default_date_format">
          <option value="YYYY-MM-DD">YYYY-MM-DD</option>
          <option value="MM/DD/YYYY">MM/DD/YYYY</option>
          <option value="DD/MM/YYYY">DD/MM/YYYY</option>
          <option value="DD.MM.YYYY">DD.MM.YYYY</option>
        </select>
      </div>

      <div class="form-actions">
        <button class="btn btn-secondary" @click="goBack">Cancel</button>
        <button
          class="btn btn-primary"
          @click="saveUIConfig"
          :disabled="isSubmitting"
        >
          {{ isSubmitting ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>

    <!-- System Settings -->
    <div v-else-if="settingId === 'system_config'" class="form">
      <div class="form-group">
        <label for="main-currency">Main Currency</label>
        <Dropdown
          id="main-currency"
          v-model="systemConfig.main_currency"
          :options="currencies"
          optionLabel="label"
          optionValue="id"
          placeholder="Select a currency"
          class="w-100 form-control currency-dropdown"
          :filter="true"
          :showClear="false"
          aria-label="Currency"
          :pt="{
            item: { class: 'custom-dropdown-item' },
            itemGroup: { class: 'custom-dropdown-group' },
            list: { class: 'custom-dropdown-list' }
          }"
        />
      </div>

      <div class="form-actions">
        <button class="btn btn-secondary" @click="goBack">Cancel</button>
        <button
          class="btn btn-primary"
          @click="saveSystemConfig"
          :disabled="isSubmitting"
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
          class="form-control json-editor"
          v-model="jsonValue"
          :class="{ 'is-invalid': jsonError }"
          rows="10"
        ></textarea>
        <div v-if="jsonError" class="error-message">{{ jsonError }}</div>
      </div>

      <div class="form-actions">
        <button class="btn btn-secondary" @click="goBack">Cancel</button>
        <button
          class="btn btn-primary"
          @click="saveCustomSetting"
          :disabled="isSubmitting || jsonError"
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
import NotificationBanner from '@/components/NotificationBanner.vue'
import Dropdown from 'primevue/dropdown'

const route = useRoute()
const router = useRouter()
const settingId = computed(() => route.params.id as string)

const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const jsonValue = ref<string>('')
const jsonError = ref<string | null>(null)
const settingData = ref<any>(null)
const currencies = ref<any[]>([])

// Track if we've loaded settings
const settingsLoaded = ref(false)

// Check if settings are required based on query parameter
const isSettingsRequired = computed(() => {
  return route.query.required === 'true'
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
      } catch (e) {
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
  }
}

function goBack() {
  router.push('/settings')
}

// Removed saveCurrencyConfig and saveTLSConfig functions as requested

async function saveUIConfig() {
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
  isSubmitting.value = true
  try {
    // Update main currency
    await settingsService.updateMainCurrency(systemConfig.value.main_currency)

    // Note: other system config fields are not in the settings model, so we don't update them

    // Redirect to settings with success message
    router.push({
      path: '/settings',
      query: { success: 'true' }
    })
  } catch (err: any) {
    error.value = 'Failed to save System config: ' + (err.message || 'Unknown error')
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

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 Bytes'

  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
</script>

<style lang="scss" scoped>
@import '@/assets/main.scss';

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

  .form {
    background-color: #f9f9f9;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);

    .form-group {
      margin-bottom: 15px;

      label {
        display: block;
        margin-bottom: 5px;
        font-weight: 500;
      }

      .form-control {
        width: 100%;
        padding: 8px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 1rem;

        &.is-invalid {
          border-color: $error-color;
        }
      }

      .json-editor {
        font-family: monospace;
        min-height: 200px;
      }

      small {
        display: block;
        margin-top: 5px;
        color: $text-secondary-color;
      }

      .error-message {
        color: $error-color;
        margin-top: 5px;
        font-size: 0.9rem;
      }

      // PrimeVue Dropdown styling - basic component styling only
      // (Global styles are in App.vue)
      :deep(.p-dropdown) {
        width: 100%;
        border: 1px solid $border-color;
        border-radius: $default-radius;
        background-color: white;
        transition: border-color 0.2s, box-shadow 0.2s;

        .p-dropdown-label {
          padding: 0.75rem;
          font-size: 1rem;
          color: $text-color;
        }

        .p-dropdown-trigger {
          width: 3rem;
          color: $text-secondary-color;
        }

        &:hover {
          border-color: darken($border-color, 10%);
        }

        &:not(.p-disabled).p-focus {
          border-color: $primary-color;
          box-shadow: 0 0 0 2px rgba($primary-color, 0.2);
          outline: none;
        }
      }
    }

    .form-actions {
      display: flex;
      justify-content: flex-end;
      gap: 10px;
      margin-top: 20px;

      button {
        padding: 8px 16px;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1rem;
        transition: background-color 0.2s;

        &.btn-primary {
          background-color: $primary-color;
          color: white;

          &:hover {
            background-color: darken($primary-color, 10%);
          }

          &:disabled {
            background-color: lighten($primary-color, 20%);
            cursor: not-allowed;
          }
        }

        &.btn-secondary {
          background-color: #6c757d;
          color: white;

          &:hover {
            background-color: darken(#6c757d, 10%);
          }
        }
      }
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

// Utility class for width 100%
.w-100 {
  width: 100%;
}

@media (max-width: 768px) {
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
