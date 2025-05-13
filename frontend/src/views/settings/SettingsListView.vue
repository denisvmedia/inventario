<template>
  <div class="settings-list">
    <div class="header">
      <h1>Settings</h1>
    </div>

    <!-- Settings Required Banner -->
    <div v-if="settingsRequired" class="settings-required-banner">
      <div class="banner-icon">
        <font-awesome-icon icon="exclamation-triangle" size="2x" />
      </div>
      <div class="banner-content">
        <h2>Settings Required</h2>
        <p>Please configure your system settings before using the application. At minimum, you need to set up:</p>
        <ul>
          <li>Main Currency</li>
        </ul>
        <p>Click on the System Settings card below to get started.</p>
      </div>
    </div>

    <!-- Settings Success Banner -->
    <div v-if="showSuccessMessage" class="settings-success-banner">
      <div class="banner-icon">
        <font-awesome-icon icon="check-circle" size="2x" />
      </div>
      <div class="banner-content">
        <h2>Settings Saved</h2>
        <p>Your settings have been successfully saved.</p>
      </div>
      <button class="close-button" @click="dismissSuccessMessage">
        <font-awesome-icon icon="times" />
      </button>
    </div>

    <div v-if="loading" class="loading">
      Loading settings...
    </div>
    <div v-else-if="error" class="error">
      {{ error }}
    </div>
    <div v-else>
      <div class="settings-categories">

        <div class="settings-category">
          <h2>User Interface</h2>
          <div class="settings-card" @click="navigateToSetting('ui_config')">
            <div class="settings-card-content">
              <h3>UI Settings</h3>
              <p>Configure theme, language, and display options</p>
              <div class="settings-values" v-if="!loading && settings.Theme">
                <div class="setting-value">
                  <span class="setting-label">Theme:</span>
                  <span class="setting-data">{{ settings.Theme }}</span>
                </div>
                <div class="setting-value">
                  <span class="setting-label">Show Debug Info:</span>
                  <span class="setting-data">{{ settings.ShowDebugInfo ? 'Yes' : 'No' }}</span>
                </div>
                <div class="setting-value" v-if="settings.DefaultDateFormat">
                  <span class="setting-label">Date Format:</span>
                  <span class="setting-data">{{ settings.DefaultDateFormat }}</span>
                </div>
              </div>
            </div>
            <div class="settings-card-icon">
              <font-awesome-icon icon="chevron-right" />
            </div>
          </div>
        </div>

        <div class="settings-category">
          <h2>System</h2>
          <div class="settings-card" @click="navigateToSetting('system_config')">
            <div class="settings-card-content">
              <h3>System Settings</h3>
              <p>Configure main currency</p>
              <div class="settings-values" v-if="!loading && settings.MainCurrency">
                <div class="setting-value">
                  <span class="setting-label">Main Currency:</span>
                  <span class="setting-data">{{ settings.MainCurrency }}</span>
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
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import settingsService from '@/services/settingsService'
import settingsCheckService from '@/services/settingsCheckService'

const router = useRouter()
const route = useRoute()
const settings = ref<any>({})
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const showSuccessMessage = ref<boolean>(false)

// Check if settings are required based on query parameter
const settingsRequired = computed(() => {
  return route.query.required === 'true' || !settings.value.MainCurrency
})

// Watch for success query parameter
watch(() => route.query.success, (success) => {
  if (success === 'true') {
    showSuccessMessage.value = true
    // Auto-dismiss after 5 seconds
    setTimeout(() => {
      showSuccessMessage.value = false
      // Remove the success query parameter
      if (route.query.success) {
        router.replace({ query: { ...route.query, success: undefined } })
      }
    }, 5000)
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

// Filter out the built-in settings
const builtInSettings = ['ui_config', 'system_config']

onMounted(async () => {
  try {
    // Get the settings from the API
    const response = await settingsService.getSettings()
    // Store the settings data
    settings.value = response.data
  } catch (err: any) {
    error.value = 'Failed to load settings: ' + (err.message || 'Unknown error')
    console.error('Error loading settings:', err)
  } finally {
    loading.value = false
  }
})

const navigateToSetting = (id: string) => {
  router.push(`/settings/${id}`)
}

const formatSettingName = (id: string) => {
  // Convert snake_case or kebab-case to Title Case
  return id
    .replace(/[-_]/g, ' ')
    .replace(/\w\S*/g, (txt) => txt.charAt(0).toUpperCase() + txt.substr(1).toLowerCase())
}
</script>

<style lang="scss" scoped>
@import '@/assets/main.scss';

.settings-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;

    h1 {
      margin: 0;
      color: $primary-color;
    }
  }

  .settings-categories {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 20px;
  }

  .settings-category {
    background-color: #f9f9f9;
    border-radius: 8px;
    padding: 15px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);

    h2 {
      margin-top: 0;
      margin-bottom: 15px;
      color: $secondary-color;
      font-size: 1.2rem;
      border-bottom: 1px solid #eee;
      padding-bottom: 8px;
    }
  }

  .settings-card {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    background-color: white;
    border-radius: 6px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
    cursor: pointer;
    transition: transform 0.2s, box-shadow 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
    }

    &:last-child {
      margin-bottom: 0;
    }

    .settings-card-content {
      h3 {
        margin: 0 0 5px 0;
        font-size: 1rem;
        color: $text-color;
      }

      p {
        margin: 0 0 10px 0;
        font-size: 0.9rem;
        color: $text-secondary-color;
      }

      .settings-values {
        margin-top: 10px;
        padding-top: 10px;
        border-top: 1px dashed #eee;

        .setting-value {
          display: flex;
          margin-bottom: 5px;
          font-size: 0.85rem;

          .setting-label {
            font-weight: 500;
            margin-right: 5px;
            color: $text-secondary-color;
          }

          .setting-data {
            color: $primary-color;
          }
        }
      }
    }

    .settings-card-icon {
      color: $secondary-color;
      margin-top: 5px;
    }
  }

  .settings-required-banner {
    display: flex;
    background-color: #fff3cd;
    border: 1px solid #ffeeba;
    border-radius: 8px;
    padding: 20px;
    margin-bottom: 20px;
    box-shadow: $box-shadow;

    .banner-icon {
      color: #856404;
      margin-right: 20px;
      display: flex;
      align-items: flex-start;
      padding-top: 5px;
    }

    .banner-content {
      flex: 1;

      h2 {
        color: #856404;
        margin-top: 0;
        margin-bottom: 10px;
        font-size: 1.25rem;
      }

      p {
        margin-bottom: 10px;
      }

      ul {
        margin-bottom: 10px;
        padding-left: 20px;
      }

      li {
        margin-bottom: 5px;
      }
    }
  }

  .settings-success-banner {
    display: flex;
    background-color: #d4edda;
    border: 1px solid #c3e6cb;
    border-radius: 8px;
    padding: 15px 20px;
    margin-bottom: 20px;
    box-shadow: $box-shadow;
    position: relative;

    .banner-icon {
      color: #155724;
      margin-right: 20px;
      display: flex;
      align-items: center;
    }

    .banner-content {
      flex: 1;

      h2 {
        color: #155724;
        margin-top: 0;
        margin-bottom: 5px;
        font-size: 1.25rem;
      }

      p {
        margin-bottom: 0;
        color: #155724;
      }
    }

    .close-button {
      background: none;
      border: none;
      color: #155724;
      cursor: pointer;
      padding: 5px;
      position: absolute;
      top: 10px;
      right: 10px;
      opacity: 0.7;

      &:hover {
        opacity: 1;
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
  .settings-list {
    .settings-categories {
      grid-template-columns: 1fr;
    }
  }
}
</style>
