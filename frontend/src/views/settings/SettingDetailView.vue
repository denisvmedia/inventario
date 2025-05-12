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
    <div v-else>
      <!-- Removed Currency Config and TLS Config as requested -->

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

        <!-- Removed Default Currency and Default Language as requested -->

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
          <label for="upload-limit">Upload Size Limit (bytes)</label>
          <input
            type="number"
            id="upload-limit"
            class="form-control"
            v-model="systemConfig.upload_size_limit"
            min="1048576"
          />
          <small>{{ formatBytes(systemConfig.upload_size_limit) }}</small>
        </div>

        <div class="form-group">
          <label for="log-level">Log Level</label>
          <select id="log-level" class="form-control" v-model="systemConfig.log_level">
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
        </div>

        <div class="form-group">
          <label for="backup-enabled">Enable Automatic Backups</label>
          <input
            type="checkbox"
            id="backup-enabled"
            v-model="systemConfig.backup_enabled"
          />
        </div>

        <div class="form-group" v-if="systemConfig.backup_enabled">
          <label for="backup-interval">Backup Interval</label>
          <select id="backup-interval" class="form-control" v-model="systemConfig.backup_interval">
            <option value="1h">Every Hour</option>
            <option value="6h">Every 6 Hours</option>
            <option value="12h">Every 12 Hours</option>
            <option value="24h">Every Day</option>
            <option value="168h">Every Week</option>
            <option value="720h">Every Month</option>
          </select>
        </div>

        <div class="form-group" v-if="systemConfig.backup_enabled">
          <label for="backup-location">Backup Location</label>
          <input
            type="text"
            id="backup-location"
            class="form-control"
            v-model="systemConfig.backup_location"
            placeholder="/path/to/backup/directory"
          />
        </div>

        <div class="form-group">
          <label for="main-currency">Main Currency</label>
          <select id="main-currency" class="form-control" v-model="systemConfig.main_currency">
            <option value="USD">US Dollar (USD)</option>
            <option value="EUR">Euro (EUR)</option>
            <option value="GBP">British Pound (GBP)</option>
            <option value="JPY">Japanese Yen (JPY)</option>
            <option value="CAD">Canadian Dollar (CAD)</option>
            <option value="AUD">Australian Dollar (AUD)</option>
            <option value="CHF">Swiss Franc (CHF)</option>
            <option value="CNY">Chinese Yuan (CNY)</option>
            <option value="INR">Indian Rupee (INR)</option>
            <option value="RUB">Russian Ruble (RUB)</option>
          </select>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import settingsService from '@/services/settingsService'

const route = useRoute()
const router = useRouter()
const settingId = computed(() => route.params.id as string)

const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const jsonValue = ref<string>('')
const jsonError = ref<string | null>(null)
const settingData = ref<any>(null)

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

onMounted(async () => {
  await loadSetting()
})

async function loadSetting() {
  loading.value = true
  error.value = null

  try {
    const response = await settingsService.getSetting(settingId.value)
    settingData.value = response.data.data.attributes.value

    if (settingId.value === 'ui_config') {
      // Parse UI config
      const config = JSON.parse(new TextDecoder().decode(settingData.value))
      uiConfig.value = {
        theme: config.theme || 'light',
        show_debug_info: config.show_debug_info || false,
        default_page_size: config.default_page_size || 20,
        default_date_format: config.default_date_format || 'YYYY-MM-DD'
      }
    } else if (settingId.value === 'system_config') {
      // Parse System config
      const config = JSON.parse(new TextDecoder().decode(settingData.value))
      systemConfig.value = {
        upload_size_limit: config.upload_size_limit || 10485760,
        log_level: config.log_level || 'info',
        backup_enabled: config.backup_enabled || false,
        backup_interval: config.backup_interval || '24h',
        backup_location: config.backup_location || '',
        main_currency: config.main_currency || 'USD'
      }
    } else {
      // For custom settings, just show the raw JSON
      jsonValue.value = JSON.stringify(JSON.parse(new TextDecoder().decode(settingData.value)), null, 2)
    }
  } catch (err: any) {
    if (err.response && err.response.status === 404) {
      // Setting doesn't exist yet, use defaults
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
    } else {
      error.value = 'Failed to load setting: ' + (err.message || 'Unknown error')
      console.error('Error loading setting:', err)
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
    await settingsService.updateUIConfig(uiConfig.value)
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
    await settingsService.updateSystemConfig(systemConfig.value)
    router.push('/settings')
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
    const value = JSON.parse(jsonValue.value)
    await settingsService.updateSetting(settingId.value, value)
    router.push('/settings')
  } catch (err: any) {
    error.value = 'Failed to save setting: ' + (err.message || 'Unknown error')
    console.error('Error saving setting:', err)
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
