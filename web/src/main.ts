import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import vuetify from './plugins/vuetify'
import { useUserStore } from './stores/user'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia).use(router).use(vuetify)

// Resolve auth state before the first navigation guard runs.
const user = useUserStore(pinia)
user.fetch().finally(() => {
  app.mount('#app')
})
