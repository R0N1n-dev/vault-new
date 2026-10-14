<script setup lang="ts">
import { computed } from 'vue'
import type { Strength } from '../types'

const props = defineProps<{
  strength: Strength | null
  /** Hides the entropy line for compact placements. */
  compact?: boolean
}>()

const SEGMENTS = [0, 1, 2, 3, 4]

const score = computed(() => Math.min(Math.max(props.strength?.score ?? 0, 0), 4))
const tone = computed(() => {
  if (!props.strength) return 'idle'
  if (score.value <= 1) return 'bad'
  if (score.value === 2) return 'warn'
  if (score.value === 3) return 'good'
  return 'great'
})
const label = computed(() => props.strength?.label ?? '—')
const bits = computed(() => props.strength?.entropyBits ?? 0)
</script>

<template>
  <div class="meter" :class="tone">
    <div class="bars" role="img" :aria-label="`Password strength: ${label}`">
      <span v-for="segment in SEGMENTS" :key="segment" class="bar" :class="{ filled: segment <= score && strength }" />
    </div>
    <div class="meta">
      <span class="label">{{ label }}</span>
      <span v-if="!compact && strength" class="faint bits">≈ {{ bits }} bits</span>
    </div>
    <p v-if="strength?.warning" class="warning">{{ strength.warning }}</p>
    <ul v-if="!compact && strength?.suggestions?.length" class="suggestions">
      <li v-for="suggestion in strength.suggestions" :key="suggestion">{{ suggestion }}</li>
    </ul>
  </div>
</template>

<style scoped>
.meter {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.bars {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 4px;
}

.bar {
  height: 4px;
  border-radius: 99px;
  background: var(--surface-3);
  transition: background 0.2s ease;
}

.meter.bad .bar.filled {
  background: var(--bad);
}

.meter.warn .bar.filled {
  background: var(--warn);
}

.meter.good .bar.filled {
  background: var(--accent);
}

.meter.great .bar.filled {
  background: var(--good);
}

.meta {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  font-size: 11.5px;
}

.label {
  font-weight: 700;
  color: var(--text-dim);
}

.meter.bad .label {
  color: var(--bad);
}

.meter.warn .label {
  color: var(--warn);
}

.meter.great .label {
  color: var(--good);
}

.bits {
  font-family: var(--mono);
}

.warning {
  font-size: 12px;
  color: var(--bad);
}

.suggestions {
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
  color: var(--text-mute);
}

.suggestions li {
  margin-top: 2px;
}
</style>
