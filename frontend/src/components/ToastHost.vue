<script setup lang="ts">
import { toasts, dismissToast } from '../stores/toasts'
import Icon from './Icon.vue'
</script>

<template>
  <Teleport to="body">
    <div class="toast-host">
      <TransitionGroup name="toast">
        <div v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.kind">
          <Icon
            :name="toast.kind === 'success' ? 'check' : toast.kind === 'error' ? 'alert' : 'info'"
            :size="16"
          />
          <div class="grow">
            <div class="title">{{ toast.title }}</div>
            <div v-if="toast.detail" class="detail">{{ toast.detail }}</div>
          </div>
          <button class="btn icon" type="button" title="Dismiss" @click="dismissToast(toast.id)">
            <Icon name="close" :size="14" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-host {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 60;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: min(380px, calc(100vw - 36px));
  pointer-events: none;
}

.toast {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 12px 12px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--border-strong);
  background: var(--surface);
  box-shadow: var(--shadow-2);
  pointer-events: auto;
}

.toast.success {
  border-color: var(--border-strong);
  border-color: color-mix(in srgb, var(--good) 45%, transparent);
}

.toast.success .title {
  color: var(--good);
}

.toast.error {
  border-color: var(--border-strong);
  border-color: color-mix(in srgb, var(--bad) 45%, transparent);
}

.toast.error .title {
  color: var(--bad);
}

.toast.info .title {
  color: var(--text);
}

.toast > .icon {
  margin-top: 1px;
}

.title {
  font-size: 13px;
  font-weight: 700;
}

.detail {
  font-size: 12.5px;
  color: var(--text-dim);
  word-break: break-word;
}

.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(20px);
}
</style>
