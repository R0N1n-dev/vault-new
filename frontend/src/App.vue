<script setup lang="ts">
import { onMounted } from 'vue'
import ToastHost from './components/ToastHost.vue'
import SetupView from './views/SetupView.vue'
import UnlockView from './views/UnlockView.vue'
import VaultView from './views/VaultView.vue'
import { bootstrap, loading, screen } from './stores/session'

onMounted(() => {
  void bootstrap()
})
</script>

<template>
  <div class="app">
    <div v-if="loading" class="boot">
      <span class="ring" />
      <p class="faint">Opening Coffer…</p>
    </div>
    <SetupView v-else-if="screen === 'setup'" />
    <UnlockView v-else-if="screen === 'unlock'" />
    <VaultView v-else />
    <ToastHost />
  </div>
</template>

<style scoped>
.app {
  height: 100%;
}

.boot {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  font-size: 13px;
}

.ring {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  border: 2.5px solid var(--surface-3);
  border-top-color: var(--accent);
  animation: spin 0.85s linear infinite;
}
</style>
