import { createApp } from 'vue'
import PrimeVue from 'primevue/config';
import ToastService from 'primevue/toastservice';
import Select from 'primevue/select'
import ToggleSwitch from 'primevue/toggleswitch'
import Dialog from 'primevue/dialog'
import DatePicker from 'primevue/datepicker'
import Toast from 'primevue/toast'
// import Aura from '@primeuix/themes/aura';
// import Nora from '@primeuix/themes/nora';
// import Lara from '@primeuix/themes/lara';
// import Material from '@primeuix/themes/material';
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './assets/main.scss'
import './assets/primevue-dropdown.scss'
import './assets/primevue-toggleswitch.scss'
import './assets/primevue-dialog.scss'
import './assets/primevue-datepicker.scss'
import './assets/primevue-toast.scss'
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
app.use(ToastService)
app.component('Select', Select)
app.component('ToggleSwitch', ToggleSwitch)
app.component('Dialog', Dialog)
app.component('DatePicker', DatePicker)
app.component('Toast', Toast)

// Register Font Awesome component globally
app.component('FontAwesomeIcon', FontAwesomeIcon)

// Initialize authentication before mounting
async function initializeApp() {
  console.log('Initializing authentication...')
  const authStore = useAuthStore()
  await authStore.initializeAuth()

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
