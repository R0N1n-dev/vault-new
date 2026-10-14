<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { friendlyError } from '../lib/backend'
import { formatDay } from '../lib/format'
import { appInfo, backendKind, busy, importVault, resetPreview, status, unlock } from '../stores/session'
import Icon from '../components/Icon.vue'
import SecretField from '../components/SecretField.vue'

const password = ref('')
const error = ref('')
const shaking = ref(false)
const passwordField = ref<HTMLInputElement | null>(null)

const info = computed(() => status.value?.info)
const corrupt = computed(() => status.value?.corrupt ?? false)

onMounted(() => {
  window.setTimeout(() => {
    const input = document.querySelector<HTMLInputElement>('input[type="password"]')
    if (input) {
      input.focus()
      passwordField.value = input
    }
  }, 60)
})

async function submit(): Promise<void> {
  if (password.value === '' || busy.value) return
  error.value = ''
  try {
    await unlock(password.value)
    password.value = ''
  } catch (caught) {
    error.value = friendlyError(caught)
    shaking.value = true
    window.setTimeout(() => {
      shaking.value = false
    }, 500)
  }
}
</script>

<template>
  <div class="screen">
    <div class="panel card" :class="{ shake: shaking }">
      <div class="mark">
        <Icon name="lock" :size="24" />
      </div>
      <h1>Welcome back</h1>
      <p class="lede">Unlock {{ appInfo?.name ?? 'your vault' }} with your master password.</p>

      <div v-if="corrupt" class="banner bad">
        <Icon name="alert" :size="16" />
        <div>
          <strong>This vault file could not be read.</strong>
          <div class="small">{{ status?.message ?? 'It may have been modified or truncated.' }}</div>
        </div>
      </div>

      <div v-if="info?.hint" class="hint-box">
        <Icon name="info" :size="14" />
        <span>Hint: <strong>{{ info.hint }}</strong></span>
      </div>

      <form class="stack" @submit.prevent="submit">
        <SecretField
          v-model="password"
          label="Master password"
          placeholder="Your master password"
          autocomplete="current-password"
          :invalid="Boolean(error)"
        />
        <p v-if="error" class="error-text">
          <Icon name="alert" :size="14" />
          {{ error }}
        </p>
        <button class="btn primary block" type="submit" :disabled="password === '' || busy">
          <Icon name="unlock" :size="16" />
          {{ busy ? 'Deriving keys…' : 'Unlock' }}
        </button>
      </form>

      <div class="meta">
        <div>
          <span class="faint">File</span>
          <code class="mono">{{ info?.path ?? appInfo?.vaultPath ?? '—' }}</code>
        </div>
        <div class="meta-row">
          <span><span class="faint">Created</span> {{ formatDay(info?.createdAt) }}</span>
          <span><span class="faint">Revisions</span> {{ info?.revision ?? '—' }}</span>
        </div>
        <div class="meta-row">
          <span><span class="faint">Key derivation</span> {{ info ? `${info.memoryMiB} MiB · ${info.passes} passes` : '—' }}</span>
        </div>
      </div>

      <div class="foot">
        <button v-if="backendKind === 'native'" class="btn ghost small" type="button" @click="importVault">
          <Icon name="upload" :size="13" />
          Open a different vault file
        </button>
        <button v-else class="btn ghost small" type="button" @click="resetPreview">
          <Icon name="refresh" :size="13" />
          Forget the preview vault
        </button>
      </div>
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
  width: min(430px, 100%);
  padding: 30px 30px 22px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  box-shadow: var(--shadow-2);
  animation: fade-in 0.22s ease;
}

.panel.shake {
  animation: shake 0.42s ease;
}

.mark {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border-radius: 14px;
  background: linear-gradient(160deg, var(--surface-3), var(--surface-2));
  border: 1px solid var(--border-strong);
  color: var(--accent-strong);
}

h1 {
  font-size: 21px;
  font-weight: 700;
}

.lede {
  font-size: 13.5px;
  color: var(--text-dim);
  line-height: 1.6;
}

.hint-box {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  background: var(--warn-soft);
  color: var(--warn);
  font-size: 12.5px;
}

.small {
  font-size: 12px;
  opacity: 0.9;
  margin-top: 2px;
}

.meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 12px;
  color: var(--text-dim);
}

.meta > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
  word-break: break-all;
}

.meta code {
  font-size: 11px;
}

.meta-row {
  flex-direction: row !important;
  justify-content: space-between;
  gap: 14px !important;
}

.foot {
  display: flex;
  justify-content: center;
}
</style>
