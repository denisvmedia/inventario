<template>
  <div class="settings-list">
    <div class="header">
      <h1>Settings</h1>
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
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import settingsService from '@/services/settingsService'

const router = useRouter()
const settings = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// Filter out the built-in settings
const builtInSettings = ['ui_config', 'system_config']

onMounted(async () => {
  try {
    const response = await settingsService.getSettings()
    // Filter out built-in settings that we handle separately
    settings.value = response.data.data.filter((setting: any) =>
      !builtInSettings.includes(setting.id)
    )
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
    align-items: center;
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
        margin: 0;
        font-size: 0.9rem;
        color: $text-secondary-color;
      }
    }

    .settings-card-icon {
      color: $secondary-color;
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
