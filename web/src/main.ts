import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './styles/mc.css'

createApp(App).use(router).mount('#app')
