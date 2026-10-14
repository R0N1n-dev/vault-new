<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { friendlyError } from '../lib/backend'
import { resolveBackend } from '../lib/backendFactory'
import { defaultGeneratorOptions, type Generated, type GeneratorOptions } from '../types'
import Icon from './Icon.vue'
import Modal from './Modal.vue'
import StrengthMeter from './StrengthMeter.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    options: GeneratorOptions
    title?: string
    acceptLabel?: string
  }>(),
  { title: 'Generate a password', acceptLabel: 'Use this password' },
)

const emit = defineEmits<{ (event: 'close'): void; (event: 'use', value: string): void }>()

const backend = resolveBackend()
const opts = ref<GeneratorOptions>({ ...defaultGeneratorOptions(), ...props.options })
const result = ref<Generated | null>(null)
const error = ref('')
const revealed = ref(true)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    opts.value = { ...defaultGeneratorOptions(), ...props.options }
    revealed.value = true
    void run()
  },
)

async function run(): Promise<void> {
  error.value = ''
  try {
    result.value = await backend.generatePassword(opts.value)
  } catch (caught) {
    result.value = null
    error.value = friendlyError(caught)
  }
}

watch(
  () => [
    opts.value.mode,
    opts.value.length,
    opts.value.words,
    opts.value.separator,
    opts.value.uppercase,
    opts.value.lowercase,
    opts.value.digits,
    opts.value.symbols,
    opts.value.excludeAmbiguous,
    opts.value.requireEverySelected,
    opts.value.capitalize,
    opts.value.appendNumber,
  ],
  () => {
    void run()
  },
)

const entropyLabel = computed(() => {
  if (!result.value) return ''
  return `${result.value.entropyBits} bits of entropy`
})

async function copy(): Promise<void> {
  if (!result.value) return
  try {
    await navigator.clipboard.writeText(result.value.value)
  } catch {
    error.value = 'The clipboard is not available in this window'
  }
}

function use(): void {
  if (!result.value) return
  emit('use', result.value.value)
  emit('close')
}
</script>

<template>
  <Modal :open="open" :title="title" subtitle="Generated on this machine, never sent anywhere." width="560px" @close="emit('close')">
    <div class="stack">
      <div class="preview">
        <code class="value mono">{{ revealed && result ? result.value : '•'.repeat(result?.value.length ?? 0) }}</code>
        <div class="preview-actions">
          <button class="btn icon" type="button" :title="revealed ? 'Hide' : 'Reveal'" @click="revealed = !revealed">
            <Icon :name="revealed ? 'eyeOff' : 'eye'" :size="16" />
          </button>
          <button class="btn icon" type="button" title="Copy" @click="copy">
            <Icon name="copy" :size="16" />
          </button>
          <button class="btn icon" type="button" title="Regenerate" @click="run">
            <Icon name="refresh" :size="16" />
          </button>
        </div>
      </div>

      <StrengthMeter v-if="result" :strength="result.strength" />
      <p v-if="entropyLabel" class="faint entropy">{{ entropyLabel }}</p>
      <p v-if="error" class="error-text">
        <Icon name="alert" :size="14" />
        {{ error }}
      </p>

      <div class="segmented" role="tablist">
        <button
          type="button"
          class="segment"
          :class="{ active: opts.mode === 'password' }"
          role="tab"
          @click="opts.mode = 'password'"
        >
          Random password
        </button>
        <button
          type="button"
          class="segment"
          :class="{ active: opts.mode === 'passphrase' }"
          role="tab"
          @click="opts.mode = 'passphrase'"
        >
          Passphrase
        </button>
      </div>

      <template v-if="opts.mode === 'password'">
        <div class="field">
          <label>Length <span class="faint">{{ opts.length }}</span></label>
          <input v-model.number="opts.length" class="range" type="range" min="8" max="64" step="1" />
        </div>
        <div class="options">
          <label class="checkbox"><input v-model="opts.uppercase" type="checkbox" /> A–Z</label>
          <label class="checkbox"><input v-model="opts.lowercase" type="checkbox" /> a–z</label>
          <label class="checkbox"><input v-model="opts.digits" type="checkbox" /> 0–9</label>
          <label class="checkbox"><input v-model="opts.symbols" type="checkbox" /> !@#$</label>
        </div>
        <div class="options">
          <label class="checkbox"><input v-model="opts.excludeAmbiguous" type="checkbox" /> Avoid look-alikes (Il1O0)</label>
          <label class="checkbox"><input v-model="opts.requireEverySelected" type="checkbox" /> Use every selected set</label>
        </div>
      </template>

      <template v-else>
        <div class="field">
          <label>Words <span class="faint">{{ opts.words }}</span></label>
          <input v-model.number="opts.words" class="range" type="range" min="3" max="12" step="1" />
        </div>
        <div class="field">
          <label>Separator</label>
          <input v-model="opts.separator" class="input mono" maxlength="3" />
        </div>
        <div class="options">
          <label class="checkbox"><input v-model="opts.capitalize" type="checkbox" /> Capitalise</label>
          <label class="checkbox"><input v-model="opts.appendNumber" type="checkbox" /> Append a digit</label>
        </div>
      </template>
    </div>

    <template #footer>
      <button class="btn" type="button" @click="emit('close')">Cancel</button>
      <button class="btn primary" type="button" :disabled="!result" @click="use">{{ acceptLabel }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.preview {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 14px 14px 16px;
  border-radius: var(--radius);
  border: 1px solid var(--border-strong);
  background: var(--surface-2);
}

.value {
  flex: 1;
  min-width: 0;
  font-size: 16px;
  letter-spacing: 0.04em;
  word-break: break-all;
  line-height: 1.4;
}

.preview-actions {
  display: flex;
  gap: 2px;
}

.entropy {
  font-size: 12px;
}

.segmented {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2px;
  padding: 3px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border);
}

.segment {
  padding: 7px 10px;
  border: 0;
  border-radius: 7px;
  background: none;
  color: var(--text-dim);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.segment.active {
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-1);
}

.options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
}

.range {
  width: 100%;
  appearance: none;
  height: 22px;
  background: none;
  cursor: pointer;
}

.range::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 99px;
  background: var(--surface-3);
}

.range::-webkit-slider-thumb {
  appearance: none;
  margin-top: -5px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--surface);
}
</style>
