<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  close: []
}>()

const close = () => emit('close')
</script>

<template>
  <div class="about-backdrop" @click.self="close">
    <div class="about-dialog">
      <h1>Vault</h1>
      <p>Local-first password vault.</p>
      <p>Wails v2 + Vue 3 + TypeScript.</p>
      <p class="version">Version 0.1.0</p>
      <button @click="close">Close</button>
    </div>
  </div>
</template>

<style scoped>
.about-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.about-dialog {
  background: #1e1e2e;
  color: white;
  padding: 2rem;
  border-radius: 8px;
  min-width: 320px;
  text-align: center;
}
.about-dialog h1 {
  margin-top: 0;
}
.about-dialog p {
  margin: 0.5rem 0;
}
.about-dialog .version {
  margin-top: 1rem;
  color: #888;
}
.about-dialog button {
  margin-top: 1rem;
  padding: 0.5rem 1rem;
}
</style>
