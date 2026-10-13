<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import UnlockScreen from './components/UnlockScreen.vue'
import VaultScreen from './components/VaultScreen.vue'
import AboutDialog from './components/AboutDialog.vue'
import { WindowSetBackgroundColour, EventsOn, EventsOff, WindowReload } from '../wailsjs/runtime/runtime'

const locked = ref(true)
const dark = ref(true)
const devtoolsOpen = ref(false)
const showAbout = ref(false)

let cleanupTheme: (() => void) | undefined
let cleanupLock: (() => void) | undefined
let cleanupDevtools: (() => void) | undefined
let cleanupAbout: (() => void) | undefined

onMounted(() => {
  WindowSetBackgroundColour(20, 20, 30, 255)
  cleanupTheme = EventsOn('menu:toggle-theme', () => {
    dark.value = !dark.value
    WindowSetBackgroundColour(
      dark.value ? 20 : 240,
      dark.value ? 20 : 240,
      dark.value ? 30 : 240,
      255
    )
  })
  cleanupLock = EventsOn('vault:lock', () => {
    locked.value = true
  })
  cleanupDevtools = EventsOn('menu:devtools', () => {
    devtoolsOpen.value = !devtoolsOpen.value
    if (devtoolsOpen.value) {
      WindowReload()
    }
  })
  cleanupAbout = EventsOn('menu:about', () => {
    showAbout.value = true
  })
})
onUnmounted(() => {
  cleanupTheme?.()
  cleanupLock?.()
  cleanupDevtools?.()
  cleanupAbout?.()
})

function toggleTheme() {
  dark.value = !dark.value
  WindowSetBackgroundColour(
    dark.value ? 20 : 240,
    dark.value ? 20 : 240,
    dark.value ? 30 : 240,
    255
  )
}
</script>

<template>
  <div class="app" :class="{ dark: dark }">
    <button class="theme-toggle" @click="toggleTheme">
      {{ dark ? 'Light' : 'Dark' }}
    </button>
    <UnlockScreen v-if="locked" />
    <VaultScreen v-else />
    <AboutDialog v-if="showAbout" @close="showAbout = false" />
  </div>
</template>

<style>
.app {
  height: 100vh;
  transition: background-color 0.3s;
}
.dark {
  background-color: rgb(20, 20, 30);
  color: white;
}
.theme-toggle {
  position: fixed;
  top: 0.5rem;
  right: 0.5rem;
  z-index: 100;
  padding: 0.3rem 0.6rem;
  border: 1px solid #666;
  border-radius: 4px;
  background: #222;
  color: white;
  cursor: pointer;
}
</style>
