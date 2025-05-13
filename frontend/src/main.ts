import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './assets/main.scss'
import { FontAwesomeIcon } from './fontawesome'

// Import PrimeVue
import PrimeVue from 'primevue/config'
// PrimeVue 4.x uses a different structure for CSS imports
import 'primeicons/primeicons.css'
// No need to import theme CSS for PrimeVue 4.x as it uses a different approach

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

app.use(router)
app.use(PrimeVue)

// Register Font Awesome component globally
app.component('font-awesome-icon', FontAwesomeIcon)

// Mount the app
console.log('Mounting Vue app to #app element')
app.mount('#app')

console.log('Vue app initialization complete')
