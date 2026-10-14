<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { formatDateTime, hostFromUrl, hueFromText, initials, relativeTime, typeLabel } from '../lib/format'
import { openExternal } from '../lib/open'
import { copy, detail, detailLoading, requestDelete, toggleFavorite } from '../stores/session'
import type { HistoryEntry, Item } from '../types'
import Icon from './Icon.vue'
import TOTPDisplay from './TOTPDisplay.vue'

const emit = defineEmits<{ (event: 'edit', item: Item): void }>()

const revealed = ref(false)
const historyOpen = ref(false)
const revealedHistory = ref<string | null>(null)
let hideTimer: number | null = null

watch(
  () => detail.value?.id,
  () => {
    revealed.value = false
    historyOpen.value = false
    revealedHistory.value = null
  },
)

function toggleReveal(): void {
  revealed.value = !revealed.value
  if (hideTimer !== null) window.clearTimeout(hideTimer)
  if (revealed.value) {
    hideTimer = window.setTimeout(() => {
      revealed.value = false
    }, 20_000)
  }
}

const item = computed(() => detail.value)
const masked = computed(() => '•'.repeat(Math.max(Math.min(item.value?.password?.length ?? 12, 20), 10)))
const host = computed(() => hostFromUrl(item.value?.url))
const history = computed<HistoryEntry[]>(() => item.value?.history ?? [])
const custom = computed(() => (item.value?.custom ?? []).filter((field) => field.label !== '' || field.value !== ''))

const cardNumber = computed(() => item.value?.card?.number ?? '')
const cardExpiry = computed(() => {
  const card = item.value?.card
  if (!card || (!card.expMonth && !card.expYear)) return ''
  const month = (card.expMonth ?? 0).toString().padStart(2, '0')
  return `${month}/${(card.expYear ?? 0).toString().slice(-2)}`
})
</script>

<template>
  <section class="detail-pane">
    <div v-if="detailLoading && !item" class="placeholder">
      <Icon name="clock" :size="22" class="spin" />
      <p class="faint">Decrypting…</p>
    </div>

    <div v-else-if="!item" class="placeholder">
      <div class="empty-mark">
        <Icon name="shieldCheck" :size="26" />
      </div>
      <p class="placeholder-title">Select an item</p>
      <p class="faint placeholder-text">
        Use the search box or the arrows to browse. Secrets are only decrypted for the item you open.
      </p>
    </div>

    <template v-else>
      <header class="head">
        <span
          class="avatar"
          :style="{ background: `linear-gradient(155deg, hsl(${hueFromText(item.title)} 62% 58%), hsl(${hueFromText(item.title) + 26} 58% 42%))` }"
        >
          {{ initials(item.title) }}
        </span>
        <div class="grow">
          <h1 class="title truncate">{{ item.title }}</h1>
          <div class="sub">
            <span class="chip">{{ typeLabel(item.type) }}</span>
            <span v-if="host" class="chip">
              <Icon name="globe" :size="11" />
              {{ host }}
            </span>
            <span v-for="tag in item.tags" :key="tag" class="chip">{{ tag }}</span>
          </div>
        </div>
        <div class="head-actions">
          <button
            class="btn icon"
            :class="{ active: item.favorite }"
            type="button"
            :title="item.favorite ? 'Remove from favorites' : 'Add to favorites'"
            @click="toggleFavorite(item.id)"
          >
            <Icon :name="item.favorite ? 'starFilled' : 'star'" :size="16" />
          </button>
          <button class="btn icon" type="button" title="Edit this item (E)" @click="emit('edit', item)">
            <Icon name="pencil" :size="16" />
          </button>
          <button class="btn icon" type="button" title="Delete this item" @click="requestDelete(item)">
            <Icon name="trash" :size="16" />
          </button>
        </div>
      </header>

      <div class="body">
        <div v-if="item.username" class="field-row">
          <div class="label">
            <span class="eyebrow">Username</span>
          </div>
          <div class="value-row">
            <span class="value mono truncate">{{ item.username }}</span>
            <button class="btn icon" type="button" title="Copy the username" @click="copy(item.username ?? '', 'Username')">
              <Icon name="copy" :size="15" />
            </button>
          </div>
        </div>

        <div v-if="item.password" class="field-row">
          <div class="label"><span class="eyebrow">Password</span></div>
          <div class="value-row">
            <span class="value mono" :class="{ masked: !revealed }">{{ revealed ? item.password : masked }}</span>
            <button class="btn icon" type="button" :title="revealed ? 'Hide' : 'Reveal'" @click="toggleReveal">
              <Icon :name="revealed ? 'eyeOff' : 'eye'" :size="15" />
            </button>
            <button class="btn icon" type="button" title="Copy the password" @click="copy(item.password ?? '', 'Password')">
              <Icon name="copy" :size="15" />
            </button>
          </div>
        </div>

        <div v-if="item.url" class="field-row">
          <div class="label"><span class="eyebrow">Website</span></div>
          <div class="value-row">
            <a class="value link truncate" :href="item.url" @click.prevent="openExternal(item.url ?? '')">
              {{ item.url }}
              <Icon name="external" :size="13" />
            </a>
            <button class="btn icon" type="button" title="Copy the address" @click="copy(item.url ?? '', 'Address')">
              <Icon name="copy" :size="15" />
            </button>
          </div>
        </div>

        <TOTPDisplay v-if="item.totp" :item-id="item.id" />

        <template v-if="item.card">
          <div class="field-row">
            <div class="label"><span class="eyebrow">Card number</span></div>
            <div class="value-row">
              <span class="value mono">{{ item.card.number || '—' }}</span>
              <button
                v-if="item.card.number"
                class="btn icon"
                type="button"
                title="Copy the card number"
                @click="copy(item.card?.number ?? '', 'Card number')"
              >
                <Icon name="copy" :size="15" />
              </button>
            </div>
          </div>
          <div class="field-row two-up">
            <div>
              <div class="label"><span class="eyebrow">Cardholder</span></div>
              <span class="value">{{ item.card.cardholder || '—' }}</span>
            </div>
            <div>
              <div class="label"><span class="eyebrow">Expires</span></div>
              <span class="value mono">{{ cardExpiry || '—' }}</span>
            </div>
            <div v-if="item.card.cvc">
              <div class="label"><span class="eyebrow">CVC</span></div>
              <span class="value mono">•••</span>
            </div>
          </div>
        </template>

        <div v-if="custom.length > 0" class="field-row">
          <div class="label"><span class="eyebrow">Custom fields</span></div>
          <div class="custom-list">
            <div v-for="(field, index) in custom" :key="`${field.label}-${index}`" class="custom-row">
              <span class="custom-label truncate">{{ field.label }}</span>
              <span class="value mono truncate">{{ field.secret ? '••••••••' : field.value }}</span>
              <button class="btn icon" type="button" title="Copy" @click="copy(field.value, field.label)">
                <Icon name="copy" :size="14" />
              </button>
            </div>
          </div>
        </div>

        <div v-if="item.notes" class="field-row">
          <div class="label">
            <span class="eyebrow">Notes</span>
            <button class="btn icon" type="button" title="Copy the notes" @click="copy(item.notes ?? '', 'Notes')">
              <Icon name="copy" :size="13" />
            </button>
          </div>
          <pre class="notes">{{ item.notes }}</pre>
        </div>

        <div v-if="history.length > 0" class="field-row">
          <button class="history-toggle" type="button" @click="historyOpen = !historyOpen">
            <Icon :name="historyOpen ? 'chevronDown' : 'chevronRight'" :size="14" />
            <span class="eyebrow">Password history</span>
            <span class="badge">{{ history.length }}</span>
          </button>
          <div v-if="historyOpen" class="history">
            <div v-for="entry in history" :key="entry.changedAt" class="history-row">
              <span class="value mono truncate">
                {{ revealedHistory === entry.changedAt ? entry.password : '••••••••••' }}
              </span>
              <span class="faint when">{{ relativeTime(entry.changedAt) }}</span>
              <button
                class="btn icon"
                type="button"
                :title="revealedHistory === entry.changedAt ? 'Hide' : 'Reveal'"
                @click="revealedHistory = revealedHistory === entry.changedAt ? null : entry.changedAt"
              >
                <Icon :name="revealedHistory === entry.changedAt ? 'eyeOff' : 'eye'" :size="14" />
              </button>
              <button class="btn icon" type="button" title="Copy" @click="copy(entry.password, 'Previous password')">
                <Icon name="copy" :size="14" />
              </button>
            </div>
          </div>
        </div>

        <div class="metadata">
          <span><Icon name="clock" :size="12" /> Created {{ formatDateTime(item.createdAt) }}</span>
          <span><Icon name="pencil" :size="12" /> Updated {{ formatDateTime(item.updatedAt) }}</span>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.detail-pane {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg);
  background: color-mix(in srgb, var(--surface) 40%, transparent);
}

.placeholder {
  margin: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 30px;
  text-align: center;
  color: var(--text-mute);
  max-width: 340px;
}

.empty-mark {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: 16px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  color: var(--accent-strong);
}

.placeholder-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-dim);
}

.placeholder-text {
  font-size: 12.5px;
  line-height: 1.6;
}

.head {
  display: flex;
  align-items: center;
  gap: 13px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.head .avatar {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  font-size: 14px;
}

.title {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.sub {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 5px;
}

.head-actions {
  display: flex;
  gap: 2px;
  flex: none;
}

.body {
  flex: 1;
  overflow: auto;
  padding: 18px 20px 26px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.field-row {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.field-row.two-up {
  flex-direction: row;
  gap: 26px;
  flex-wrap: wrap;
}

.field-row.two-up > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.label {
  display: flex;
  align-items: center;
  gap: 8px;
}

.value-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 10px 9px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-2);
}

.value {
  flex: 1;
  min-width: 0;
  font-size: 13.5px;
}

.value.masked {
  letter-spacing: 0.16em;
  color: var(--text-dim);
}

.link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--accent-strong);
}

.link:hover {
  text-decoration: underline;
}

.custom-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.custom-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-2);
}

.custom-label {
  flex: none;
  width: 34%;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-dim);
}

.notes {
  margin: 0;
  padding: 12px 13px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-2);
  font-family: var(--font);
  font-size: 13px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-dim);
}

.history-toggle {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 4px 0;
  border: 0;
  background: none;
  color: var(--text-mute);
  cursor: pointer;
}

.history-toggle:hover {
  color: var(--text-dim);
}

.history {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.history-row {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border);
}

.when {
  font-size: 11px;
  flex: none;
}

.metadata {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding-top: 6px;
  border-top: 1px solid var(--border);
  font-size: 11.5px;
  color: var(--text-mute);
}

.metadata span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
