<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { resolveBackend } from '../lib/backendFactory'
import { closeEditor, editing, effectiveSettings, saveItem } from '../stores/session'
import { normalizeItem } from '../types'
import type { Card, CustomField, DraftItem, Item, ItemType, Strength } from '../types'
import GeneratorPanel from './GeneratorPanel.vue'
import Icon from './Icon.vue'
import Modal from './Modal.vue'
import SecretField from './SecretField.vue'
import StrengthMeter from './StrengthMeter.vue'
import TagInput from './TagInput.vue'

const draft = ref<DraftItem | null>(null)
const strength = ref<Strength | null>(null)
const titleTouched = ref(false)
const totpError = ref('')
const generatorOpen = ref(false)
let strengthTimer: number | null = null

watch(
  editing,
  (value) => {
    if (!value) {
      draft.value = null
      return
    }
    const clone = normalizeItem(JSON.parse(JSON.stringify(value)) as Item)
    if (clone.type === 'card' && !clone.card) clone.card = {}
    draft.value = clone
    titleTouched.value = false
    totpError.value = ''
    void scoreNow()
  },
  { immediate: true },
)

const item = computed(() => draft.value)
/** Card is always present for card items, which keeps the template simple. */
const card = computed<Card>(() => draft.value?.card ?? {})
const tags = computed<string[]>({
  get: () => draft.value?.tags ?? [],
  set: (value) => {
    if (draft.value) draft.value.tags = value
  },
})
const open = computed(() => draft.value !== null)
const isNew = computed(() => !draft.value?.id)
const titleError = computed(() => titleTouched.value && (draft.value?.title.trim() ?? '') === '')
const totpLooksWrong = computed(() => {
  const value = (draft.value?.totp ?? '').trim()
  if (value === '') return false
  if (value.toLowerCase().startsWith('otpauth://')) return false
  return !/^[A-Za-z2-7\s-]{16,}$/.test(value)
})
const canSave = computed(() => Boolean(draft.value && draft.value.title.trim() !== '' && !totpLooksWrong.value))

async function scoreNow(): Promise<void> {
  const password = draft.value?.password ?? ''
  if (password === '' && draft.value?.type !== 'login') {
    strength.value = null
    return
  }
  strength.value = await resolveBackend().scorePassword(password)
}

function onPasswordInput(): void {
  if (strengthTimer !== null) window.clearTimeout(strengthTimer)
  strengthTimer = window.setTimeout(() => {
    void scoreNow()
  }, 160)
  totpError.value = ''
}

function setType(type: ItemType): void {
  if (!draft.value) return
  draft.value.type = type
  if (type === 'card' && !draft.value.card) draft.value.card = {}
  void scoreNow()
}

function updateCustom(index: number, patch: Partial<CustomField>): void {
  const list = draft.value?.custom
  if (!list || !list[index]) return
  list[index] = { ...list[index], ...patch } as CustomField
}

function addCustom(): void {
  draft.value?.custom?.push({ label: '', value: '', secret: false })
}

function removeCustom(index: number): void {
  draft.value?.custom?.splice(index, 1)
}

/** formatCardNumber groups digits in fours, which is how cards are printed. */
function formatCardNumber(input: string): string {
  const digits = input.replace(/[^0-9]/g, '').slice(0, 19)
  return digits.replace(/(.{4})/g, '$1 ').trim()
}

function detectBrand(digits: string): string {
  if (/^4/.test(digits)) return 'Visa'
  if (/^(5[1-5]|2[2-7])/.test(digits)) return 'Mastercard'
  if (/^3[47]/.test(digits)) return 'American Express'
  if (/^(6011|65|64[4-9])/.test(digits)) return 'Discover'
  if (/^3(0[0-5]|[68])/.test(digits)) return 'Diners Club'
  if (/^35/.test(digits)) return 'JCB'
  return ''
}

function onCardNumber(value: string): void {
  if (!draft.value) return
  const formatted = formatCardNumber(value)
  const digits = formatted.replace(/\s/g, '')
  const next: Card = { ...(draft.value.card ?? {}), number: formatted }
  if (!next.brand) {
    const brand = detectBrand(digits)
    if (brand) next.brand = brand
  }
  draft.value.card = next
}

async function save(): Promise<void> {
  titleTouched.value = true
  if (!draft.value || !canSave.value) return
  if (totpLooksWrong.value) {
    totpError.value = 'That does not look like a base32 secret or otpauth link'
    return
  }
  // The store closes the editor and surfaces any backend validation error.
  await saveItem(draft.value)
}

/** The template cannot reach `navigator`, so the copy action lives here. */
function copyValue(value: string): void {
  void navigator.clipboard.writeText(value)
}

function useGenerated(value: string): void {
  if (!draft.value) return
  draft.value.password = value
  void scoreNow()
}
</script>

<template>
  <Modal
    :open="open"
    :title="isNew ? 'Add an item' : 'Edit item'"
    :subtitle="isNew ? 'Everything is encrypted before it is written to disk.' : undefined"
    width="620px"
    @close="closeEditor"
  >
    <div v-if="item" class="stack">
      <div class="segmented">
        <button
          v-for="option in (['login', 'note', 'card'] as ItemType[])"
          :key="option"
          type="button"
          class="segment"
          :class="{ active: item.type === option }"
          @click="setType(option)"
        >
          <Icon :name="option === 'login' ? 'key' : option === 'note' ? 'note' : 'card'" :size="14" />
          {{ option === 'login' ? 'Login' : option === 'note' ? 'Secure note' : 'Card' }}
        </button>
      </div>

      <div class="field">
        <label>Title <span v-if="titleError" class="error-text">required</span></label>
        <input
          v-model="item.title"
          class="input"
          :class="{ invalid: titleError }"
          placeholder="e.g. GitHub"
          autocomplete="off"
          @blur="titleTouched = true"
        />
      </div>

      <template v-if="item.type === 'login'">
        <div class="field">
          <label>Username or email</label>
          <input v-model="item.username" class="input" autocomplete="off" spellcheck="false" />
        </div>
        <SecretField
          v-model="item.password"
          label="Password"
          generatable
          copyable
          :strength="strength"
          @generate="generatorOpen = true"
          @copy="copyValue"
          @update:model-value="onPasswordInput"
        />
        <div class="field">
          <label>Website</label>
          <input v-model="item.url" class="input" placeholder="https://" autocomplete="off" spellcheck="false" />
        </div>
        <div class="field">
          <label>One-time password secret</label>
          <input
            v-model="item.totp"
            class="input mono"
            :class="{ invalid: totpLooksWrong }"
            placeholder="Base32 secret or otpauth:// link"
            autocomplete="off"
            spellcheck="false"
          />
          <p v-if="totpLooksWrong" class="error-text">
            <Icon name="alert" :size="13" />
            That does not look like a base32 secret or an otpauth link.
          </p>
          <p v-else-if="totpError" class="error-text">
            <Icon name="alert" :size="13" />
            {{ totpError }}
          </p>
          <p v-else class="hint">Codes are generated locally. Nothing is sent to a server.</p>
        </div>
      </template>

      <template v-else-if="item.type === 'card'">
        <div class="field">
          <label>Cardholder</label>
          <input v-model="card.cardholder" class="input" autocomplete="off" />
        </div>
        <div class="field">
          <label>Number</label>
          <input
            :value="card.number ?? ''"
            class="input mono"
            inputmode="numeric"
            autocomplete="off"
            placeholder="0000 0000 0000 0000"
            @input="onCardNumber(($event.target as HTMLInputElement).value)"
          />
          <p v-if="card.brand" class="hint">Detected: {{ card.brand }}</p>
        </div>
        <div class="grid-3">
          <div class="field">
            <label>Expiry month</label>
            <input v-model.number="card.expMonth" class="input" type="number" min="1" max="12" />
          </div>
          <div class="field">
            <label>Expiry year</label>
            <input v-model.number="card.expYear" class="input" type="number" min="2000" max="2099" />
          </div>
          <div class="field">
            <label>CVC</label>
            <input v-model="card.cvc" class="input mono" inputmode="numeric" maxlength="4" autocomplete="off" />
          </div>
        </div>
      </template>

      <SecretField
        v-model="item.notes"
        :label="item.type === 'note' ? 'Note' : 'Notes'"
        :secret="false"
        multiline
        copyable
        :rows="item.type === 'note' ? 8 : 4"
        placeholder="Anything worth remembering about this item."
      />

      <div class="field">
        <label>Custom fields</label>
        <div v-if="(item.custom ?? []).length > 0" class="custom-editor">
          <div v-for="(field, index) in item.custom" :key="index" class="custom-editor-row">
            <input
              :value="field.label"
              class="input small-input"
              placeholder="Label"
              @input="updateCustom(index, { label: ($event.target as HTMLInputElement).value })"
            />
            <input
              :value="field.value"
              class="input small-input mono"
              placeholder="Value"
              @input="updateCustom(index, { value: ($event.target as HTMLInputElement).value })"
            />
            <label class="checkbox secret-toggle" title="Mark as secret">
              <input
                type="checkbox"
                :checked="field.secret"
                @change="updateCustom(index, { secret: ($event.target as HTMLInputElement).checked })"
              />
            </label>
            <button class="btn icon" type="button" title="Remove" @click="removeCustom(index)">
              <Icon name="close" :size="14" />
            </button>
          </div>
        </div>
        <button class="btn small quiet align-start" type="button" @click="addCustom">
          <Icon name="plus" :size="13" />
          Add a field
        </button>
      </div>

      <div class="field">
        <label>Tags</label>
        <TagInput v-model="tags" placeholder="work, finance…" />
      </div>

      <StrengthMeter v-if="item.type === 'login' && item.password" :strength="strength" />
    </div>

    <template #footer>
      <button class="btn" type="button" @click="closeEditor">Cancel</button>
      <button class="btn primary" type="button" :disabled="!canSave" @click="save">
        <Icon name="check" :size="15" />
        {{ isNew ? 'Add item' : 'Save changes' }}
      </button>
    </template>
  </Modal>

  <GeneratorPanel
    :open="generatorOpen"
    :options="effectiveSettings.generator"
    accept-label="Use this password"
    @close="generatorOpen = false"
    @use="useGenerated"
  />
</template>

<style scoped>
.segmented {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 3px;
  padding: 3px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border);
}

.segment {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 6px;
  border: 0;
  border-radius: 7px;
  background: none;
  color: var(--text-dim);
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
}

.segment.active {
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-1);
}

.grid-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.custom-editor {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.custom-editor-row {
  display: grid;
  grid-template-columns: 1fr 1.4fr auto auto;
  gap: 6px;
  align-items: center;
}

.small-input {
  padding: 7px 9px;
  font-size: 12.5px;
}

.secret-toggle {
  padding: 0 2px;
}

.align-start {
  align-self: flex-start;
}
</style>
