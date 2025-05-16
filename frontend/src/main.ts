import { createApp } from 'vue'
import PrimeVue from 'primevue/config';
import Select from 'primevue/select'
import InputSwitch from 'primevue/inputswitch'
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

app.use(router)
app.use(PrimeVue, {
  // unstyled: true,
  ripple: false,
  // theme: {
  //   preset: Aura
  // }
})
app.component('Select', Select)
app.component('InputSwitch', InputSwitch)

// Register Font Awesome component globally
app.component('FontAwesomeIcon', FontAwesomeIcon)

// Mount the app
console.log('Mounting Vue app to #app element')
app.mount('#app')

console.log('Vue app initialization complete')
