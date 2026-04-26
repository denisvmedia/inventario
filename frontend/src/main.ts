import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './assets/app.css'
import './assets/main.scss'
import { initThemeOnBoot } from '@design/composables/useTheme'
import { initDensityOnBoot } from '@design/composables/useDensity'

initThemeOnBoot()
initDensityOnBoot()

console.log('Initializing Vue application')
console.log('Router:', router)
console.log('Available routes:', router.getRoutes().map(route => ({
  path: route.path,
  name: route.name
})))

const app = createApp(App)

app.config.errorHandler = (err, instance, info) => {
  console.error('Vue Error:', err)
  console.error('Error Info:', info)
  console.error('Component:', instance)
}

const pinia = createPinia()
app.use(pinia)

import { useAuthStore } from './stores/authStore'

app.use(router)

async function initializeApp() {
  console.log('Initializing authentication...')
  const authStore = useAuthStore()

  await authStore.initializeAuth()

  console.log('Auth initialization complete, auth state:', {
    isAuthenticated: authStore.isAuthenticated,
    isInitialized: authStore.isInitialized,
    user: authStore.user?.email || 'none'
  })

  console.log('Mounting Vue app to #app element')
  app.mount('#app')

  console.log('Vue app initialization complete')
}

initializeApp().catch(error => {
  console.error('Failed to initialize app:', error)
  app.mount('#app')
})
