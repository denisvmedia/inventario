<template>
  <div class="system-page">
    <div class="header">
      <h1>System</h1>
    </div>

    <!-- Settings Required Banner -->
    <div v-if="settingsRequired && settingsLoaded" class="settings-required-banner">
      <div class="banner-icon">
        <font-awesome-icon icon="exclamation-triangle" size="2x" />
      </div>
      <div class="banner-content">
        <h2>Settings Required</h2>
        <p class="banner-text">Please configure your system settings before using the application. At minimum, you need to set up:</p>
        <ul>
          <li>Main Currency</li>
        </ul>
        <p class="banner-text">Click on the System Settings card below to get started.</p>
      </div>
    </div>

    <!-- Success Message -->
    <div v-if="showSuccessMessage" class="success-message">
      <div class="success-content">
        <font-awesome-icon icon="check-circle" />
        <span>Settings updated successfully!</span>
        <button @click="dismissSuccessMessage" class="dismiss-btn">
          <font-awesome-icon icon="times" />
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading system information...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else class="system-content">

      <!-- System Information Section -->
      <div class="system-section">
        <h2>System Information</h2>
        <div class="info-cards">

          <!-- Version Information Card -->
          <div class="info-card">
            <div class="info-card-header">
              <h3><font-awesome-icon icon="code-branch" /> Version Information</h3>
            </div>
            <div class="info-card-content">
              <div class="info-item">
                <span class="info-label">Version:</span>
                <span class="info-value">{{ systemInfo.version }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Build Date:</span>
                <span class="info-value">{{ systemInfo.build_date }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Commit:</span>
                <span class="info-value">{{ systemInfo.commit }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Go Version:</span>
                <span class="info-value">{{ systemInfo.go_version }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Platform:</span>
                <span class="info-value">{{ systemInfo.platform }}</span>
              </div>
            </div>
          </div>

          <!-- System Backend Card -->
          <div class="info-card">
            <div class="info-card-header">
              <h3><font-awesome-icon icon="server" /> System Backend</h3>
            </div>
            <div class="info-card-content">
              <div class="info-item">
                <span class="info-label">Database:</span>
                <span class="info-value">{{ systemInfo.database_backend }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">File Storage:</span>
                <span class="info-value">{{ systemInfo.file_storage_backend }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Operating System:</span>
                <span class="info-value">{{ systemInfo.operating_system }}</span>
              </div>
            </div>
          </div>

          <!-- Runtime Metrics Card -->
          <div class="info-card">
            <div class="info-card-header">
              <h3><font-awesome-icon icon="chart-line" /> Runtime Metrics</h3>
            </div>
            <div class="info-card-content">
              <div class="info-item">
                <span class="info-label">Uptime:</span>
                <span class="info-value">{{ systemInfo.uptime }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Memory Usage:</span>
                <span class="info-value">{{ systemInfo.memory_usage }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Goroutines:</span>
                <span class="info-value">{{ systemInfo.num_goroutines }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">CPU Cores:</span>
                <span class="info-value">{{ systemInfo.num_cpu }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Settings Section -->
      <div class="system-section">
        <h2>Settings</h2>
        <div class="settings-categories">

          <div class="settings-category">
            <div class="settings-category-title">User Interface</div>
            <div class="settings-card" @click="navigateToSetting('ui_config')">
              <div class="settings-card-content">
                <h4>UI Settings</h4>
                <p>Configure theme, language, and display options</p>
                <div v-if="!loading && systemInfo.settings.Theme" class="settings-values">
                  <div class="setting-value">
                    <span class="setting-label">Theme:</span>
                    <span class="setting-data">{{ systemInfo.settings.Theme }}</span>
                  </div>
                  <div class="setting-value">
                    <span class="setting-label">Show Debug Info:</span>
                    <span class="setting-data">{{ systemInfo.settings.ShowDebugInfo ? 'Yes' : 'No' }}</span>
                  </div>
                  <div v-if="systemInfo.settings.DefaultDateFormat" class="setting-value">
                    <span class="setting-label">Date Format:</span>
                    <span class="setting-data">{{ systemInfo.settings.DefaultDateFormat }}</span>
                  </div>
                </div>
              </div>
              <div class="settings-card-icon">
                <font-awesome-icon icon="chevron-right" />
              </div>
            </div>
          </div>

          <div class="settings-category">
            <div class="settings-category-title">System</div>
            <div class="settings-card" @click="navigateToSetting('system_config')">
              <div class="settings-card-content">
                <h4>System Settings</h4>
                <p>Configure main currency</p>
                <div v-if="!loading && systemInfo.settings.MainCurrency" class="settings-values">
                  <div class="setting-value">
                    <span class="setting-label">Main Currency:</span>
                    <span class="setting-data">{{ systemInfo.settings.MainCurrency }}</span>
                  </div>
                </div>
              </div>
              <div class="settings-card-icon">
                <font-awesome-icon icon="chevron-right" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import systemService, { type SystemInfo } from '@/services/systemService'

const router = useRouter()
const route = useRoute()
const systemInfo = ref<SystemInfo>({
  version: '',
  commit: '',
  build_date: '',
  go_version: '',
  platform: '',
  database_backend: '',
  file_storage_backend: '',
  operating_system: '',
  uptime: '',
  memory_usage: '',
  num_goroutines: 0,
  num_cpu: 0,
  settings: {}
})
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const showSuccessMessage = ref<boolean>(false)

// Track if we've loaded settings
const settingsLoaded = ref(false)

// Check if settings are required based on actual state
const settingsRequired = computed(() => {
  // Only check URL parameter if settings haven't been loaded yet
  if (!settingsLoaded.value) {
    return route.query.required === 'true'
  }
  // After settings are loaded, check actual state: settings are required only if MainCurrency is not set
  return !systemInfo.value.settings.MainCurrency
})

// Watch for success query parameter
watch(() => route.query.success, (newValue) => {
  if (newValue === 'true') {
    showSuccessMessage.value = true
  }
}, { immediate: true })

// Function to dismiss the success message
const dismissSuccessMessage = () => {
  showSuccessMessage.value = false
  // Remove the success query parameter
  if (route.query.success) {
    router.replace({ query: { ...route.query, success: undefined } })
  }
}

onMounted(async () => {
  try {
    // Get the system information from the API
    const response = await systemService.getSystemInfo()
    // Store the system information
    systemInfo.value = response.data
  } catch (err: any) {
    error.value = 'Failed to load system information: ' + (err.message || 'Unknown error')
    console.error('Error loading system information:', err)
  } finally {
    loading.value = false
    // Mark settings as loaded to prevent flashing the banner
    settingsLoaded.value = true
  }
})

const navigateToSetting = (id: string) => {
  router.push(`/system/settings/${id}`)
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.system-page {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;

    h1 {
      margin: 0;
      color: $text-color;
    }
  }

  .system-content {
    display: flex;
    flex-direction: column;
    gap: 40px;
  }

  .system-section {
    h2 {
      margin-top: 0;
      margin-bottom: 1.5rem;
      color: $text-color;
      font-size: 1.5rem;
      border-bottom: 1px solid #eee;
      padding-bottom: 0.5rem;
    }
  }

  // System Information Section
  .info-cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
    gap: 1.5rem;
  }

  .info-card {
    background: white;
    border-radius: $default-radius;
    padding: 1.5rem;
    box-shadow: $box-shadow;

    .info-card-header {
      margin-bottom: 1rem;

      h3 {
        margin: 0;
        margin-bottom: 1rem;
        padding-bottom: 0.5rem;
        border-bottom: 1px solid #eee;
        font-size: 1.1rem;
        color: $text-color;
        display: flex;
        align-items: center;
        gap: 8px;
      }
    }

    .info-card-content {
      padding: 0;
    }

    .info-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 0.75rem;

      &:last-child {
        margin-bottom: 0;
      }

      .info-label {
        font-weight: 500;
        color: $text-color;
        width: 140px;
      }

      .info-value {
        font-family: monospace;
        background-color: $light-bg-color;
        padding: 0.25rem 0.5rem;
        border-radius: $default-radius;
        font-size: 0.9rem;
        color: $text-color;
      }
    }
  }

  // Settings Section
  .settings-categories {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 1.5rem;
  }

  .settings-category {
    .settings-category-title {
      margin-top: 0;
      margin-bottom: 1rem;
      color: $text-color;
      font-size: 1.2rem;
      border-bottom: 1px solid #eee;
      padding-bottom: 0.5rem;
    }
  }

  .settings-card {
    background: white;
    border-radius: $default-radius;
    padding: 1.5rem;
    box-shadow: $box-shadow;
    cursor: pointer;
    transition: transform 0.2s, box-shadow 0.2s;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;

    &:hover {
      transform: translateY(-5px);
      box-shadow: 0 5px 15px rgb(0 0 0 / 10%);
    }

    .settings-card-content {
      flex: 1;

      h4 {
        margin: 0 0 0.5rem;
        color: $text-color;
        font-size: 1.1rem;
        font-weight: 600;
      }

      p {
        margin: 0 0 1rem;
        color: $text-secondary-color;
        font-size: 0.9rem;
        line-height: 1.4;
      }

      .settings-values {
        .setting-value {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 0.5rem;
          font-size: 0.85rem;

          &:last-child {
            margin-bottom: 0;
          }

          .setting-label {
            color: $text-secondary-color;
            font-weight: 500;
          }

          .setting-data {
            font-weight: 600;
            color: $text-color;
            background-color: $light-bg-color;
            padding: 0.25rem 0.5rem;
            border-radius: $default-radius;
          }
        }
      }
    }

    .settings-card-icon {
      color: $text-secondary-color;
      font-size: 1.2rem;
      margin-left: 1rem;
    }
  }
}

// Shared styles from original settings page
.settings-required-banner {
  background: linear-gradient(135deg, #ff6b6b, #ee5a24);
  color: white;
  padding: 20px;
  border-radius: 8px;
  margin-bottom: 20px;
  display: flex;
  align-items: flex-start;
  gap: 15px;
  box-shadow: 0 4px 6px rgb(0 0 0 / 10%);

  .banner-icon {
    font-size: 2rem;
    margin-top: 5px;
  }

  .banner-content {
    flex: 1;

    h2 {
      margin: 0 0 10px;
      font-size: 1.5rem;
    }

    .banner-text {
      margin: 0 0 10px;
      line-height: 1.5;
    }

    ul {
      margin: 10px 0;
      padding-left: 20px;

      li {
        margin-bottom: 5px;
      }
    }
  }
}

.success-message {
  background-color: #d4edda;
  border: 1px solid #c3e6cb;
  color: #155724;
  padding: 15px;
  border-radius: 8px;
  margin-bottom: 20px;

  .success-content {
    display: flex;
    align-items: center;
    gap: 10px;

    .dismiss-btn {
      background: none;
      border: none;
      color: #155724;
      cursor: pointer;
      margin-left: auto;
      padding: 5px;
      border-radius: 4px;

      &:hover {
        background-color: #c3e6cb;
      }
    }
  }
}

.loading, .error {
  text-align: center;
  padding: 40px;
  font-size: 1.1rem;
}

.error {
  color: #dc3545;
  background-color: #f8d7da;
  border: 1px solid #f5c6cb;
  border-radius: 8px;
}
</style>
