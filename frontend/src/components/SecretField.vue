<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from './Icon.vue'
import StrengthMeter from './StrengthMeter.vue'
import type { Strength } from '../types'

const props = withDefaults(
  defineProps<{
    modelValue: string
    label?: string
    placeholder?: string
    autocomplete?: string
    /** Shows the reveal toggle. */
    secret?: boolean
    /** Shows a dice button that asks the parent to generate a value. */
    generatable?: boolean
    /** Shows a copy button. */
    copyable?: boolean
    /** Renders a strength meter for the current value. */
    strength?: Strength | null
    hint?: string
    invalid?: boolean
    multiline?: boolean
    rows?: number
    disabled?: boolean
  }>(),
  { secret: true, rows: 4, invalid: false },
)

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
  (event: 'generate'): void
  (event: 'copy', value: string): void
}>()

const revealed = ref(false)
let hideTimer: number | null = null

function clearHideTimer(): void {
  if (hideTimer !== null) {
    window.clearTimeout(hideTimer)
    hideTimer = null
  }
}

/** Revealed secrets hide themselves again so they cannot linger on screen. */
function toggleReveal(): void {
  revealed.value = !revealed.value
  clearHideTimer()
  if (revealed.value) {
    hideTimer = window.setTimeout(() => {
      revealed.value = false
    }, 20_000)
  }
}

watch(
  () => props.modelValue,
  () => {
    if (revealed.value && props.secret) {
      clearHideTimer()
      hideTimer = window.setTimeout(() => {
        revealed.value = false
      }, 20_000)
    }
  },
)

onBeforeUnmount(clearHideTimer)

const inputType = computed(() => (props.secret && !revealed.value ? 'password' : 'text'))
const showHint = computed(() => Boolean(props.hint) && !props.invalid)
</script>

<template>
  <div class="field">
    <label v-if="label">
      {{ label }}
      <span v-if="invalid" class="error-text">required</span>
    </label>

    <textarea
      v-if="multiline"
      class="textarea mono"
      :class="{ invalid }"
      :value="modelValue"
      :placeholder="placeholder"
      :rows="rows"
      :disabled="disabled"
      spellcheck="false"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    />
    <div v-else class="input-group">
      <input
        class="input"
        :class="{ mono: true, invalid }"
        :type="inputType"
        :value="modelValue"
        :placeholder="placeholder"
        :autocomplete="autocomplete ?? 'off'"
        :disabled="disabled"
        spellcheck="false"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      />
      <div class="actions">
        <button
          v-if="generatable"
          class="btn icon"
          type="button"
          title="Generate a password"
          @click="emit('generate')"
        >
          <Icon name="sparkles" :size="15" />
        </button>
        <button
          v-if="copyable"
          class="btn icon"
          type="button"
          title="Copy"
          :disabled="!modelValue"
          @click="emit('copy', modelValue)"
        >
          <Icon name="copy" :size="15" />
        </button>
        <button
          v-if="secret"
          class="btn icon"
          type="button"
          :title="revealed ? 'Hide' : 'Reveal'"
          @click="toggleReveal"
        >
          <Icon :name="revealed ? 'eyeOff' : 'eye'" :size="15" />
        </button>
      </div>
    </div>

    <StrengthMeter v-if="strength" :strength="strength" compact />
    <p v-if="showHint" class="hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.input-group .input {
  padding-right: 96px;
  font-family: var(--mono);
  letter-spacing: 0.02em;
}
</style>
