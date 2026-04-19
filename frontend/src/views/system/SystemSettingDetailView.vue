<template>
  <div class="setting-detail">
    <div class="header">
      <div class="back-link">
        <router-link to="/system">
          <font-awesome-icon icon="arrow-left" /> Back to System
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

    <!-- UI Settings -->
    <div v-if="settingId === 'ui_config'" class="form">
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

  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import settingsService from '@/services/settingsService'

import Select from 'primevue/select'

const route = useRoute()
const router = useRouter()
const settingId = computed(() => route.params.id as string)

const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)

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

// UI Config
const uiConfig = ref({
  theme: 'light',
  show_debug_info: false,
  default_page_size: 20,
  default_date_format: 'YYYY-MM-DD'
})

// Form validation errors
const formErrors = ref({
  theme: '',
  default_date_format: ''
})

const settingTitle = computed(() => {
  switch (settingId.value) {
    case 'ui_config':
      return 'User Interface Settings'
    default:
      return formatSettingName(settingId.value)
  }
})

onMounted(async () => {
  await loadSetting()
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
    }
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/system')
}

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

    router.push('/system')
  } catch (err: any) {
    error.value = 'Failed to save UI config: ' + (err.message || 'Unknown error')
    console.error('Error saving UI config:', err)
  } finally {
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

// Currency change hint styling
.currency-change-hint,
.field-help {
  color: $secondary-color;
  font-size: 0.875rem;
  margin-top: 0.5rem;
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
