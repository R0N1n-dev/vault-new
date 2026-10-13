<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useVault } from '../composables/useVault'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const { locked, doUnlock } = useVault()
const pin = ref('')
const error = ref('')

let cleanup: (() => void) | undefined
onMounted(() => {
  cleanup = EventsOn('menu:unlock', () => {
    pin.value = ''
    error.value = ''
  })
})
onUnmounted(() => {
  cleanup?.()
})

async function unlock() {
  error.value = ''
  try {
    await doUnlock(pin.value)
  } catch (e) {
    error.value = String(e)
  }
}
</script>

<template>
  <div class="unlock">
    <h1>Vault</h1>
    <p>Enter your PIN to unlock</p>
    <input v-model="pin" type="password" placeholder="PIN" />
    <button @click="unlock">Unlock</button>
    <p v-if="error" class="error">{{ error }}</p>
  </div>
</template>

<style scoped>
.unlock {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100vh;
  gap: 1rem;
}
input {
  padding: 0.5rem;
  font-size: 1.2rem;
  width: 120px;
  text-align: center;
}
button {
  padding: 0.5rem 1rem;
}
.error {
  color: #f87171;
}
</style>
