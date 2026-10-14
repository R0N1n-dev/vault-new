<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import Icon from './Icon.vue'

withDefaults(
  defineProps<{
    open: boolean
    title?: string
    subtitle?: string
    width?: string
    /** Disables closing by clicking the backdrop or pressing Escape. */
    persistent?: boolean
  }>(),
  { width: '540px', persistent: false },
)

const emit = defineEmits<{ (event: 'close'): void }>()

function requestClose(): void {
  emit('close')
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <Transition name="sheet">
      <div v-if="open" class="backdrop" @mousedown.self="!persistent && requestClose()">
        <div class="sheet card" :style="{ width }" role="dialog" aria-modal="true">
          <header v-if="title || $slots.header" class="sheet-head">
            <slot name="header">
              <div class="grow">
                <h2>{{ title }}</h2>
                <p v-if="subtitle" class="muted subtitle">{{ subtitle }}</p>
              </div>
              <button v-if="!persistent" class="btn icon" type="button" title="Close" @click="requestClose">
                <Icon name="close" :size="16" />
              </button>
            </slot>
          </header>
          <div class="sheet-body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="sheet-foot">
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.backdrop {
  position: fixed;
  inset: 0;
  z-index: 40;
  display: grid;
  place-items: center;
  padding: 24px;
  background: var(--bg);
  background: color-mix(in srgb, var(--bg) 72%, transparent);
  backdrop-filter: blur(6px);
  overflow: auto;
}

.sheet {
  max-width: 100%;
  max-height: calc(100vh - 48px);
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-2);
  animation: pop-in 0.18s ease;
}

.sheet-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 18px 20px 12px;
}

.sheet-head h2 {
  font-size: 17px;
  font-weight: 700;
}

.subtitle {
  margin-top: 3px;
  font-size: 13px;
}

.sheet-body {
  padding: 4px 20px 18px;
  overflow: auto;
}

.sheet-foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 20px;
  border-top: 1px solid var(--border);
  background: var(--surface-2);
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
}

.sheet-enter-active,
.sheet-leave-active {
  transition: opacity 0.16s ease;
}

.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
}
</style>
