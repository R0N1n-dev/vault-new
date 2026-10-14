<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { friendlyError } from '../lib/backend'
import { formatDateTime, formatDay } from '../lib/format'
import {
  appInfo,
  backendKind,
  busy,
  changeMasterPassword,
  effectiveSettings,
  exportBackup,
  importVault,
  nativeDialogs,
  openImport,
  restoreBackup,
  saveSettings,
  setTheme,
  settings,
  status,
  theme,
  unlocked,
  updateHint,
} from '../stores/session'
import type { Settings, Strength, Theme } from '../types'
import { defaultSettings } from '../types'
import { resolveBackend } from '../lib/backendFactory'
import Icon from './Icon.vue'
import Modal from './Modal.vue'
import StrengthMeter from './StrengthMeter.vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()

const form = ref<Settings>({ ...defaultSettings() })
const hint = ref('')
const passwordFormOpen = ref(false)
const current = ref('')
const next = ref('')
const confirm = ref('')
const passwordError = ref('')
const passwordStrength = ref<Strength | null>(null)
const saved = ref(false)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    form.value = JSON.parse(JSON.stringify(effectiveSettings.value)) as Settings
    hint.value = status.value?.info?.hint ?? ''
    saved.value = false
    passwordFormOpen.value = false
    current.value = ''
    next.value = ''
    confirm.value = ''
    passwordError.value = ''
  },
)

const info = computed(() => status.value?.info)
const backupAvailable = computed(() => info.value?.backupAvailable ?? false)

const autoLockOptions = [
  { value: 0, label: 'Never' },
  { value: 1, label: 'After 1 minute' },
  { value: 5, label: 'After 5 minutes' },
  { value: 15, label: 'After 15 minutes' },
  { value: 30, label: 'After 30 minutes' },
  { value: 60, label: 'After 1 hour' },
]

const clipboardOptions = [0, 15, 30, 60, 120]

const trashOptions = [
  { value: 0, label: 'Keep until I remove them' },
  { value: 7, label: '7 days' },
  { value: 30, label: '30 days' },
  { value: 180, label: '6 months' },
  { value: 365, label: '1 year' },
]

const mismatch = computed(() => next.value !== '' && confirm.value !== '' && next.value !== confirm.value)
const canChangePassword = computed(
  () =>
    current.value !== '' &&
    next.value.length >= 10 &&
    !mismatch.value &&
    next.value !== current.value &&
    !busy.value,
)

async function save(): Promise<void> {
  const ok = await saveSettings(form.value)
  if (ok) {
    saved.value = true
    window.setTimeout(() => {
      saved.value = false
    }, 2200)
  }
}

async function scoreNext(): Promise<void> {
  passwordStrength.value = next.value === '' ? null : await resolveBackend().scorePassword(next.value)
}

async function submitPasswordChange(): Promise<void> {
  passwordError.value = ''
  if (!canChangePassword.value) return
  try {
    await changeMasterPassword(current.value, next.value)
    passwordFormOpen.value = false
    current.value = ''
    next.value = ''
    confirm.value = ''
  } catch (error) {
    passwordError.value = friendlyError(error)
  }
}

function pickTheme(value: Theme): void {
  form.value.theme = value
  setTheme(value)
}
</script>

<template>
  <Modal :open="open" title="Settings" subtitle="Everything here is stored inside the encrypted vault." width="660px" @close="emit('close')">
    <div class="stack">
      <section class="section">
        <div class="eyebrow">Security</div>
        <div class="grid-2">
          <div class="field">
            <label>Lock the vault</label>
            <select v-model.number="form.autoLockMinutes" class="select">
              <option v-for="option in autoLockOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </div>
          <div class="field">
            <label>Clear the clipboard after</label>
            <select v-model.number="form.clipboardClearSeconds" class="select">
              <option v-for="seconds in clipboardOptions" :key="seconds" :value="seconds">
                {{ seconds === 0 ? 'Never' : `${seconds} seconds` }}
              </option>
            </select>
          </div>
        </div>
        <label class="checkbox">
          <input v-model="form.lockOnBlur" type="checkbox" />
          Lock when the window loses focus
        </label>
        <div class="field">
          <label>Keep password history <span class="faint">{{ form.passwordHistoryMax }} entries</span></label>
          <input v-model.number="form.passwordHistoryMax" class="range" type="range" min="0" max="20" step="1" />
          <p class="hint">Previous passwords are stored inside the vault, never in plain text.</p>
        </div>
        <div class="field">
          <label>Deleted items</label>
          <select v-model.number="form.trashRetentionDays" class="select">
            <option v-for="option in trashOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
          <p class="hint">
            Deleting an item moves it to the trash instead of removing it. Expired items leave the vault file
            the next time it is saved.
          </p>
        </div>
        <div class="field">
          <label>Master password</label>
          <button class="btn quiet align-start" type="button" @click="passwordFormOpen = true">
            <Icon name="key" :size="15" />
            Change master password
          </button>
        </div>
        <div class="field">
          <label>Password hint</label>
          <div class="row">
            <input v-model="hint" class="input grow" placeholder="Something only you would understand" />
            <button class="btn quiet" type="button" :disabled="hint === (info?.hint ?? '')" @click="updateHint(hint)">
              Update
            </button>
          </div>
          <p class="hint">Shown on the unlock screen. Never put the password itself here.</p>
        </div>
      </section>

      <section class="section">
        <div class="eyebrow">Appearance</div>
        <div class="segmented">
          <button
            v-for="option in (['dark', 'light', 'system'] as Theme[])"
            :key="option"
            type="button"
            class="segment"
            :class="{ active: theme === option }"
            @click="pickTheme(option)"
          >
            <Icon :name="option === 'dark' ? 'moon' : option === 'light' ? 'sun' : 'monitor'" :size="14" />
            {{ option === 'system' ? 'Match system' : option === 'dark' ? 'Dark' : 'Light' }}
          </button>
        </div>
      </section>

      <section class="section">
        <div class="eyebrow">Default generator</div>
        <div class="segmented">
          <button
            type="button"
            class="segment"
            :class="{ active: form.generator.mode === 'password' }"
            @click="form.generator.mode = 'password'"
          >
            Random password
          </button>
          <button
            type="button"
            class="segment"
            :class="{ active: form.generator.mode === 'passphrase' }"
            @click="form.generator.mode = 'passphrase'"
          >
            Passphrase
          </button>
        </div>
        <div v-if="form.generator.mode === 'password'" class="field">
          <label>Length <span class="faint">{{ form.generator.length }}</span></label>
          <input v-model.number="form.generator.length" class="range" type="range" min="8" max="64" />
          <div class="options">
            <label class="checkbox"><input v-model="form.generator.uppercase" type="checkbox" /> A–Z</label>
            <label class="checkbox"><input v-model="form.generator.lowercase" type="checkbox" /> a–z</label>
            <label class="checkbox"><input v-model="form.generator.digits" type="checkbox" /> 0–9</label>
            <label class="checkbox"><input v-model="form.generator.symbols" type="checkbox" /> Symbols</label>
            <label class="checkbox"><input v-model="form.generator.excludeAmbiguous" type="checkbox" /> Avoid look-alikes</label>
          </div>
        </div>
        <div v-else class="field">
          <label>Words <span class="faint">{{ form.generator.words }}</span></label>
          <input v-model.number="form.generator.words" class="range" type="range" min="3" max="12" />
        </div>
      </section>

      <section class="section">
        <div class="eyebrow">Vault file</div>
        <div class="info-grid">
          <div><span class="faint">Location</span><code class="mono path">{{ info?.path ?? appInfo?.vaultPath ?? '—' }}</code></div>
          <div><span class="faint">Identifier</span><code class="mono">{{ info?.vaultId ?? '—' }}</code></div>
          <div><span class="faint">Key derivation</span><span>{{ info?.kdf ?? '—' }}</span></div>
          <div>
            <span class="faint">Cost</span>
            <span>{{ info ? `${info.memoryMiB} MiB · ${info.passes} passes · ${info.parallelism} lanes` : '—' }}</span>
          </div>
          <div><span class="faint">Revisions written</span><span>{{ info?.revision ?? '—' }}</span></div>
          <div><span class="faint">Created</span><span>{{ formatDay(info?.createdAt) }}</span></div>
          <div><span class="faint">Last saved</span><span>{{ formatDateTime(info?.updatedAt) }}</span></div>
          <div><span class="faint">Previous revision</span><span>{{ backupAvailable ? 'Available' : 'None yet' }}</span></div>
        </div>
        <div class="actions">
          <button class="btn quiet" type="button" @click="openImport">
            <Icon name="upload" :size="15" />
            Import from CSV
          </button>
          <button class="btn quiet" type="button" @click="exportBackup">
            <Icon name="download" :size="15" />
            Export encrypted backup
          </button>
          <button class="btn quiet" type="button" :disabled="!backupAvailable" @click="restoreBackup">
            <Icon name="history" :size="15" />
            Restore previous revision
          </button>
          <button v-if="nativeDialogs" class="btn quiet" type="button" :disabled="unlocked" title="Lock the vault first" @click="importVault">
            <Icon name="upload" :size="15" />
            Import a vault
          </button>
        </div>
        <p v-if="!nativeDialogs" class="hint">
          Importing and restoring need the desktop build; the browser preview keeps its vault in localStorage.
        </p>
      </section>

      <section class="section about">
        <div class="eyebrow">About</div>
        <p class="about-text">
          {{ appInfo?.name ?? 'Coffer' }} {{ appInfo?.version ?? '' }} · {{ appInfo?.platform ?? '' }} ·
          {{ backendKind === 'native' ? 'native vault' : 'browser preview' }}
        </p>
        <p class="faint about-text">
          Offline by design: no account, no sync and no network calls. Forgotten master passwords cannot be
          recovered — restore an encrypted backup instead.
        </p>
      </section>
    </div>

    <template #footer>
      <span v-if="saved" class="saved-note">
        <Icon name="check" :size="14" />
        Saved
      </span>
      <button class="btn" type="button" @click="emit('close')">Close</button>
      <button class="btn primary" type="button" :disabled="busy" @click="save">
        Save settings
      </button>
    </template>
  </Modal>

  <Modal
    :open="passwordFormOpen"
    title="Change master password"
    subtitle="The vault is re-encrypted with a fresh key derived from the new password."
    width="480px"
    @close="passwordFormOpen = false"
  >
    <div class="stack">
      <div class="field">
        <label>Current master password</label>
        <input v-model="current" class="input mono" type="password" autocomplete="current-password" />
      </div>
      <div class="field">
        <label>New master password</label>
        <input
          v-model="next"
          class="input mono"
          type="password"
          autocomplete="new-password"
          @input="scoreNext"
        />
      </div>
      <StrengthMeter :strength="passwordStrength" />
      <div class="field">
        <label>Repeat the new password</label>
        <input v-model="confirm" class="input mono" type="password" autocomplete="new-password" />
        <p v-if="mismatch" class="error-text">
          <Icon name="alert" :size="13" />
          The two passwords do not match
        </p>
      </div>
      <p class="hint">
        At least 10 characters. Older encrypted backups keep working with the previous password.
      </p>
      <p v-if="passwordError" class="error-text">
        <Icon name="alert" :size="13" />
        {{ passwordError }}
      </p>
    </div>
    <template #footer>
      <button class="btn" type="button" @click="passwordFormOpen = false">Cancel</button>
      <button class="btn primary" type="button" :disabled="!canChangePassword" @click="submitPasswordChange">
        Change password
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.section {
  display: flex;
  flex-direction: column;
  gap: 11px;
  padding: 14px 0;
  border-bottom: 1px solid var(--border);
}

.section:last-child {
  border-bottom: 0;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
}

.segmented {
  display: flex;
  gap: 3px;
  padding: 3px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border);
}

.segment {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 7px 8px;
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

.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 18px;
}

.info-grid > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12.5px;
}

.path {
  word-break: break-all;
  font-size: 11.5px;
  color: var(--text-dim);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.align-start {
  align-self: flex-start;
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

.about-text {
  font-size: 12.5px;
  line-height: 1.6;
}

.saved-note {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-right: auto;
  color: var(--good);
  font-size: 12.5px;
  font-weight: 700;
}
</style>
