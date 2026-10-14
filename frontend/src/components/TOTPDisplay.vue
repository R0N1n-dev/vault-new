<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { resolveBackend } from '../lib/backendFactory'
import { copy } from '../stores/session'
import Icon from './Icon.vue'

const props = defineProps<{ itemId: string }>()

const code = ref('')
const remaining = ref(0)
const period = ref(30)
const loading = ref(false)
const failed = ref(false)
let timer: number | null = null

const RADIUS = 15.5
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

const pretty = computed(() => {
  if (code.value.length <= 4) return code.value
  const half = Math.ceil(code.value.length / 2)
  return `${code.value.slice(0, half)} ${code.value.slice(half)}`
})

const offset = computed(() => {
  const total = period.value || 30
  const fraction = Math.max(0, Math.min(1, remaining.value / total))
  return CIRCUMFERENCE * (1 - fraction)
})

const urgent = computed(() => remaining.value <= 5)

async function refresh(): Promise<void> {
  loading.value = true
  try {
    const result = await fetchCode()
    if (!result) {
      failed.value = true
      return
    }
    code.value = result.code
    remaining.value = result.remaining
    period.value = result.period
    failed.value = false
  } finally {
    loading.value = false
  }
}

async function fetchCode(): Promise<{ code: string; remaining: number; period: number } | null> {
  try {
    const result = await resolveBackend().totpCode(props.itemId)
    return { code: result.code, remaining: result.remaining, period: result.period }
  } catch {
    return null
  }
}

function startTicker(): void {
  stopTicker()
  timer = window.setInterval(() => {
    remaining.value -= 1
    if (remaining.value <= 0) void refresh()
  }, 1000)
}

function stopTicker(): void {
  if (timer !== null) window.clearInterval(timer)
  timer = null
}

watch(
  () => props.itemId,
  () => {
    void refresh()
  },
  { immediate: true },
)

startTicker()
onBeforeUnmount(stopTicker)
</script>

<template>
  <div class="totp">
    <div class="ring-wrap">
      <svg class="ring" viewBox="0 0 36 36" :class="{ urgent }">
        <circle class="track" cx="18" cy="18" :r="RADIUS" />
        <circle
          class="progress"
          cx="18"
          cy="18"
          :r="RADIUS"
          :stroke-dasharray="CIRCUMFERENCE"
          :stroke-dashoffset="offset"
        />
      </svg>
      <span class="seconds">{{ Math.max(remaining, 0) }}</span>
    </div>
    <div class="grow">
      <div class="label eyebrow">One-time password</div>
      <div v-if="failed" class="error-text">
        <Icon name="alert" :size="14" /> This secret could not be read
      </div>
      <div v-else class="code mono" :class="{ loading }">{{ pretty || '••• •••' }}</div>
    </div>
    <div class="actions">
      <button class="btn icon" type="button" title="Refresh the code" @click="refresh">
        <Icon name="refresh" :size="16" />
      </button>
      <button class="btn icon" type="button" title="Copy the code" :disabled="!code" @click="copy(code, 'One-time password', 15)">
        <Icon name="copy" :size="16" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.totp {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--surface-2);
}

.ring-wrap {
  position: relative;
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  flex: none;
}

.ring {
  position: absolute;
  inset: 0;
  transform: rotate(-90deg);
}

.track {
  fill: none;
  stroke: var(--surface-3);
  stroke-width: 3;
}

.progress {
  fill: none;
  stroke: var(--accent);
  stroke-width: 3;
  stroke-linecap: round;
  transition: stroke-dashoffset 1s linear, stroke 0.3s ease;
}

.ring.urgent .progress {
  stroke: var(--warn);
}

.seconds {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--text-dim);
}

.code {
  font-size: 22px;
  font-weight: 600;
  letter-spacing: 0.14em;
  margin-top: 2px;
}

.code.loading {
  opacity: 0.5;
}

.actions {
  display: flex;
  gap: 2px;
}
</style>
