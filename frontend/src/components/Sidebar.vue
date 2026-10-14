<script setup lang="ts">
import { computed } from 'vue'
import { formatCountdown, pluralize } from '../lib/format'
import {
  applyFilters,
  audit,
  autoLock,
  backendKind,
  filters,
  itemCount,
  lock,
  stats,
  switchView,
  tags,
  view,
} from '../stores/session'
import Icon from './Icon.vue'

const emit = defineEmits<{ (event: 'settings'): void }>()

interface NavEntry {
  key: string
  label: string
  icon: 'shield' | 'starFilled' | 'key' | 'note' | 'card'
  count: number
  active: boolean
  select: () => void
}

const entries = computed<NavEntry[]>(() => [
  {
    key: 'all',
    label: 'All items',
    icon: 'shield',
    count: stats.value?.total ?? itemCount.value,
    active: view.value === 'items' && !filters.value.favoritesOnly && filters.value.type === '' && filters.value.tag === '',
    select: () => {
      switchView('items')
      applyFilters({ favoritesOnly: false, type: '', tag: '' })
    },
  },
  {
    key: 'favorites',
    label: 'Favorites',
    icon: 'starFilled',
    count: stats.value?.favorites ?? 0,
    active: view.value === 'items' && filters.value.favoritesOnly && filters.value.type === '',
    select: () => {
      switchView('items')
      applyFilters({ favoritesOnly: true, type: '', tag: '' })
    },
  },
  {
    key: 'logins',
    label: 'Logins',
    icon: 'key',
    count: stats.value?.logins ?? 0,
    active: view.value === 'items' && filters.value.type === 'login' && !filters.value.favoritesOnly,
    select: () => {
      switchView('items')
      applyFilters({ type: 'login', favoritesOnly: false, tag: '' })
    },
  },
  {
    key: 'notes',
    label: 'Secure notes',
    icon: 'note',
    count: stats.value?.notes ?? 0,
    active: view.value === 'items' && filters.value.type === 'note' && !filters.value.favoritesOnly,
    select: () => {
      switchView('items')
      applyFilters({ type: 'note', favoritesOnly: false, tag: '' })
    },
  },
  {
    key: 'cards',
    label: 'Cards',
    icon: 'card',
    count: stats.value?.cards ?? 0,
    active: view.value === 'items' && filters.value.type === 'card' && !filters.value.favoritesOnly,
    select: () => {
      switchView('items')
      applyFilters({ type: 'card', favoritesOnly: false, tag: '' })
    },
  },
])

/** selectTag toggles a tag filter and makes sure the item list is showing. */
function selectTag(name: string): void {
  switchView('items')
  applyFilters({ tag: filters.value.tag.toLowerCase() === name.toLowerCase() ? '' : name })
}

const scoreTone = computed(() => {
  const score = audit.value?.score
  if (score === undefined) return ''
  if (score >= 90) return 'good'
  if (score >= 70) return 'ok'
  if (score >= 40) return 'warn'
  return 'bad'
})

const lockLabel = computed(() => {
  if (!autoLock.value.enabled) return 'Auto-lock off'
  return `Locks in ${formatCountdown(autoLock.value.secondsRemaining)}`
})

const lockUrgent = computed(() => autoLock.value.enabled && autoLock.value.secondsRemaining <= 30)
</script>

<template>
  <aside class="sidebar">
    <header class="brand">
      <div class="mark">
        <Icon name="shieldCheck" :size="19" />
      </div>
      <div class="grow">
        <div class="name">Coffer</div>
        <div class="faint tagline">
          {{ backendKind === 'native' ? 'Local vault' : 'Browser preview' }}
        </div>
      </div>
    </header>

    <nav class="nav">
      <button
        v-for="entry in entries"
        :key="entry.key"
        type="button"
        class="nav-item"
        :class="{ active: entry.active }"
        @click="entry.select()"
      >
        <Icon :name="entry.icon" :size="16" />
        <span class="grow truncate">{{ entry.label }}</span>
        <span class="badge">{{ entry.count }}</span>
      </button>
    </nav>

    <div class="nav tools">
      <button
        type="button"
        class="nav-item"
        :class="{ active: view === 'audit' }"
        @click="switchView('audit')"
      >
        <Icon name="shieldCheck" :size="16" />
        <span class="grow truncate">Security check</span>
        <span v-if="audit" class="badge" :class="scoreTone">{{ audit.score }}</span>
      </button>
      <button
        type="button"
        class="nav-item"
        :class="{ active: view === 'trash' }"
        @click="switchView('trash')"
      >
        <Icon name="trash" :size="16" />
        <span class="grow truncate">Trash</span>
        <span v-if="(stats?.trashed ?? 0) > 0" class="badge">{{ stats?.trashed }}</span>
      </button>
    </div>

    <div v-if="tags.length > 0" class="tags">
      <div class="eyebrow section">Tags</div>
      <button
        v-for="tag in tags"
        :key="tag.name"
        type="button"
        class="tag-item"
        :class="{ active: filters.tag.toLowerCase() === tag.name.toLowerCase() }"
        @click="selectTag(tag.name)"
      >
        <Icon name="tag" :size="13" />
        <span class="grow truncate">{{ tag.name }}</span>
        <span class="faint count">{{ tag.count }}</span>
      </button>
    </div>

    <footer class="foot">
      <div class="status" :class="{ urgent: lockUrgent }">
        <Icon name="clock" :size="14" />
        <span>{{ lockLabel }}</span>
      </div>
      <div class="foot-actions">
        <button class="btn small quiet grow" type="button" @click="emit('settings')">
          <Icon name="settings" :size="14" />
          Settings
        </button>
        <button class="btn small quiet" type="button" title="Lock the vault now (Ctrl+L)" @click="lock()">
          <Icon name="lock" :size="14" />
          Lock
        </button>
      </div>
      <p class="faint locked-note">{{ pluralize(itemCount, 'item') }} · encrypted at rest</p>
    </footer>
  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar);
  flex: none;
  display: flex;
  flex-direction: column;
  padding: 14px 10px 12px;
  background: var(--surface);
  background: color-mix(in srgb, var(--surface) 75%, transparent);
  border-right: 1px solid var(--border);
  backdrop-filter: blur(8px);
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 8px 14px;
}

.mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: linear-gradient(160deg, var(--accent-strong), var(--accent));
  color: var(--accent-contrast);
  box-shadow: var(--shadow-1);
}

.name {
  font-size: 14.5px;
  font-weight: 700;
  letter-spacing: 0.01em;
}

.tagline {
  font-size: 11.5px;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: var(--radius-xs);
  background: none;
  color: var(--text-dim);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  transition: background 0.14s ease, color 0.14s ease;
}

.nav-item:hover {
  background: var(--surface-2);
  color: var(--text);
}

.nav-item.active {
  background: var(--accent-soft);
  color: var(--accent-strong);
}

.tools {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}

.badge.good {
  color: var(--good);
}

.badge.ok {
  color: var(--accent-strong);
}

.badge.warn {
  color: var(--warn);
}

.badge.bad {
  color: var(--bad);
}

.tags {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 1px;
  overflow: auto;
  min-height: 0;
}

.section {
  padding: 0 10px 6px;
}

.tag-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  border: 0;
  border-radius: var(--radius-xs);
  background: none;
  color: var(--text-mute);
  font-size: 12.5px;
  cursor: pointer;
  text-align: left;
}

.tag-item:hover {
  background: var(--surface-2);
  color: var(--text-dim);
}

.tag-item.active {
  background: var(--accent-soft);
  color: var(--accent-strong);
}

.count {
  font-family: var(--mono);
  font-size: 11px;
}

.foot {
  margin-top: auto;
  padding: 10px 8px 0;
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.status {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: var(--text-mute);
}

.status.urgent {
  color: var(--warn);
}

.foot-actions {
  display: flex;
  gap: 6px;
}

.locked-note {
  font-size: 11px;
  text-align: center;
}
</style>
