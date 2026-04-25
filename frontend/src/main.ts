import { createApp } from 'vue'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1331
import PrimeVue from 'primevue/config';
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1331
import Select from 'primevue/select'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1331
import ToggleSwitch from 'primevue/toggleswitch'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1331
import Dialog from 'primevue/dialog'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1331
import DatePicker from 'primevue/datepicker'
// import Aura from '@primeuix/themes/aura';
// import Nora from '@primeuix/themes/nora';
// import Lara from '@primeuix/themes/lara';
// import Material from '@primeuix/themes/material';
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
// app.css loads Tailwind v4 and design-system tokens before the legacy
// SCSS bundle so base utilities land first and PrimeVue/SCSS rules keep
// their current precedence during the Epic #1324 migration.
import './assets/app.css'
import './assets/main.scss'
import './assets/primevue-dropdown.scss'
import './assets/primevue-toggleswitch.scss'
import './assets/primevue-dialog.scss'
import './assets/primevue-datepicker.scss'
import './assets/primevue-fileupload.scss'
import './assets/primevue-progressbar.scss'
import './assets/primevue-progressspinner.scss'
import './assets/primevue-badge.scss'
import { FontAwesomeIcon } from './fontawesome'

// Add some debug logging
console.log('Initializing Vue application')
console.log('Router:', router)
console.log('Available routes:', router.getRoutes().map(route => ({
  path: route.path,
  name: route.name
})))

const app = createApp(App)

// Add error handler
app.config.errorHandler = (err, instance, info) => {
  console.error('Vue Error:', err)
  console.error('Error Info:', info)
  console.error('Component:', instance)
}

// Initialize Pinia
const pinia = createPinia()
app.use(pinia)

// Initialize authentication store
import { useAuthStore } from './stores/authStore'

app.use(router)
app.use(PrimeVue, {
  // unstyled: true,
  ripple: false,
  // theme: {
  //   preset: Aura
  // }
  locale: {
    firstDayOfWeek: 1,
  },
})
app.component('Select', Select)
app.component('ToggleSwitch', ToggleSwitch)
app.component('Dialog', Dialog)
app.component('DatePicker', DatePicker)

// Register Font Awesome component globally
app.component('FontAwesomeIcon', FontAwesomeIcon)

// Initialize authentication before mounting
async function initializeApp() {
  console.log('Initializing authentication...')
  const authStore = useAuthStore()

  // Initialize auth synchronously first (restore from localStorage)
  await authStore.initializeAuth()

  console.log('Auth initialization complete, auth state:', {
    isAuthenticated: authStore.isAuthenticated,
    isInitialized: authStore.isInitialized,
    user: authStore.user?.email || 'none'
  })

  // Mount the app
  console.log('Mounting Vue app to #app element')
  app.mount('#app')

  console.log('Vue app initialization complete')
}

// Start the app
initializeApp().catch(error => {
  console.error('Failed to initialize app:', error)
  // Mount anyway to show error state
  app.mount('#app')
})
