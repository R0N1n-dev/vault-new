import { createApp } from 'vue'
import App from './App.vue'
import { applyTheme, readStoredTheme } from './lib/theme'
import './style.css'

// Apply the stored theme before the first paint so the window never flashes the
// wrong colours.
applyTheme(readStoredTheme())

createApp(App).mount('#app')
