<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { friendlyError } from '../lib/backend'
import { resolveBackend } from '../lib/backendFactory'
import { appInfo, backendKind, busy, createVault } from '../stores/session'
import type { Strength } from '../types'
import Icon from '../components/Icon.vue'
import SecretField from '../components/SecretField.vue'
import StrengthMeter from '../components/StrengthMeter.vue'

const password = ref('')
const confirm = ref('')
const hint = ref('')
const error = ref('')
const strength = ref<Strength | null>(null)
const acceptedWeak = ref(false)
let scoreTimer: number | null = null

const MIN_LENGTH = 10

const tooShort = computed(() => password.value.length > 0 && password.value.length < MIN_LENGTH)
const mismatch = computed(() => confirm.value.length > 0 && confirm.value !== password.value)
const weak = computed(() => (strength.value?.score ?? 0) <= 1 && password.value.length >= MIN_LENGTH)
const blockedByStrength = computed(() => weak.value && !acceptedWeak.value)
const canCreate = computed(
  () =>
    password.value.length >= MIN_LENGTH &&
    confirm.value === password.value &&
    !blockedByStrength.value &&
    !busy.value,
)

watch(password, () => {
  if (scoreTimer !== null) window.clearTimeout(scoreTimer)
  scoreTimer = window.setTimeout(() => {
    void (async () => {
      strength.value = password.value === '' ? null : await resolveBackend().scorePassword(password.value)
    })()
  }, 150)
})

async function submit(): Promise<void> {
  error.value = ''
  if (!canCreate.value) return
  try {
    await createVault(password.value, hint.value)
  } catch (caught) {
    error.value = friendlyError(caught)
  }
}
</script>

<template>
  <div class="screen">
    <div class="panel card">
      <div class="mark">
        <Icon name="shieldCheck" :size="26" />
      </div>
      <h1>Create your vault</h1>
      <p class="lede">
        Coffer keeps every password in a single encrypted file on this machine. There is no account, no sync and
        no server.
      </p>

      <form class="stack" @submit.prevent="submit">
        <SecretField
          v-model="password"
          label="Master password"
          placeholder="At least 10 characters"
          autocomplete="new-password"
          :strength="strength"
          :invalid="tooShort"
        />
        <p v-if="tooShort" class="error-text">
          <Icon name="alert" :size="13" />
          Use at least {{ MIN_LENGTH }} characters.
        </p>

        <SecretField
          v-model="confirm"
          label="Repeat the master password"
          autocomplete="new-password"
          :invalid="mismatch"
        />
        <p v-if="mismatch" class="error-text">
          <Icon name="alert" :size="13" />
          The two passwords do not match.
        </p>

        <label
          v-if="weak"
          class="checkbox warning"
          title="This password can be brute-forced faster than a random one"
        >
          <input v-model="acceptedWeak" type="checkbox" />
          I understand this password is weak
        </label>

        <div class="field">
          <label>Hint <span class="faint">optional</span></label>
          <input v-model="hint" class="input" placeholder="Shown on the unlock screen" />
        </div>

        <p v-if="error" class="error-text">
          <Icon name="alert" :size="14" />
          {{ error }}
        </p>

        <button class="btn primary block" type="submit" :disabled="!canCreate">
          <Icon name="lock" :size="16" />
          {{ busy ? 'Deriving keys…' : 'Create vault' }}
        </button>
      </form>

      <p class="fine">
        <Icon name="info" :size="13" />
        Your master password cannot be recovered. If you lose it, the only way back is an encrypted backup.
      </p>
      <p class="faint where">
        {{ backendKind === 'native' ? 'Vault file' : 'Preview storage' }}:
        <code class="mono">{{ appInfo?.vaultPath ?? '…' }}</code>
      </p>
    </div>
  </div>
</template>

<style scoped>
.screen {
  height: 100%;
  display: grid;
  place-items: center;
  padding: 32px;
  overflow: auto;
}

.panel {
  width: min(460px, 100%);
  padding: 30px 30px 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  box-shadow: var(--shadow-2);
  animation: fade-in 0.25s ease;
}

.mark {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border-radius: 15px;
  background: linear-gradient(160deg, var(--accent-strong), var(--accent));
  color: var(--accent-contrast);
  box-shadow: var(--shadow-1);
}

h1 {
  font-size: 21px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.lede {
  font-size: 13.5px;
  line-height: 1.65;
  color: var(--text-dim);
  margin-bottom: 4px;
}

.warning {
  color: var(--warn);
}

.fine {
  display: flex;
  gap: 8px;
  margin-top: 4px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-mute);
}

.where {
  font-size: 11.5px;
  text-align: center;
  word-break: break-all;
}

.where code {
  font-size: 11px;
}
</style>
