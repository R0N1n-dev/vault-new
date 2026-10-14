<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { hostFromUrl, hueFromText, initials, relativeTime } from '../lib/format'
import { applyFilters, detail, filters, items, startCreate, toggleFavorite } from '../stores/session'
import type { ItemType, ListOptions } from '../types'
import Icon from './Icon.vue'

const emit = defineEmits<{ (event: 'select', id: string): void }>()

const searchField = ref<HTMLInputElement | null>(null)
const query = ref(filters.value.query)
const menuOpen = ref(false)
let debounce: number | null = null

watch(
  () => filters.value.query,
  (value) => {
    if (value !== query.value) query.value = value
  },
)

function onQueryInput(): void {
  if (debounce !== null) window.clearTimeout(debounce)
  debounce = window.setTimeout(() => {
    applyFilters({ query: query.value })
  }, 140)
}

function clearQuery(): void {
  query.value = ''
  applyFilters({ query: '' })
  searchField.value?.focus()
}

function choose(type: ItemType): void {
  menuOpen.value = false
  startCreate(type)
}

function onWindowKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') menuOpen.value = false
}

window.addEventListener('keydown', onWindowKeydown)
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onWindowKeydown)
  if (debounce !== null) window.clearTimeout(debounce)
})

const SORT_ORDER: ListOptions['sort'][] = ['title', 'updated', 'created']

function cycleSort(): void {
  const index = SORT_ORDER.indexOf(filters.value.sort)
  const next = SORT_ORDER[(index + 1) % SORT_ORDER.length] ?? 'title'
  applyFilters({ sort: next })
}

const sortLabel = computed(() => {
  switch (filters.value.sort) {
    case 'updated':
      return 'Recently changed'
    case 'created':
      return 'Recently added'
    default:
      return 'Title A–Z'
  }
})

const activeFilters = computed(
  () =>
    Boolean(filters.value.query) ||
    Boolean(filters.value.type) ||
    Boolean(filters.value.tag) ||
    filters.value.favoritesOnly,
)

defineExpose({
  focusSearch: () => searchField.value?.focus(),
  focusCreate: () => {
    menuOpen.value = true
  },
})
</script>

<template>
  <section class="list-pane">
    <div class="toolbar">
      <div class="search">
        <Icon name="search" :size="15" />
        <input
          ref="searchField"
          v-model="query"
          class="search-field"
          type="search"
          placeholder="Search titles, tags, notes…"
          autocomplete="off"
          spellcheck="false"
          @input="onQueryInput"
        />
        <button v-if="query" class="btn icon" type="button" title="Clear" @click="clearQuery">
          <Icon name="close" :size="14" />
        </button>
        <span v-else class="kbd">Ctrl K</span>
      </div>

      <div class="toolbar-row">
        <button class="btn small quiet grow" type="button" @click="cycleSort">
          <Icon name="filter" :size="14" />
          {{ sortLabel }}
        </button>
        <div class="create">
          <button class="btn small primary" type="button" @click="choose('login')">
            <Icon name="plus" :size="14" />
            New
          </button>
          <button class="btn small primary caret" type="button" title="Choose a type" @click="menuOpen = !menuOpen">
            <Icon name="chevronDown" :size="13" />
          </button>
          <Transition name="menu">
            <div v-if="menuOpen" class="menu card">
              <button type="button" class="menu-item" @click="choose('login')">
                <Icon name="key" :size="15" /> Login
              </button>
              <button type="button" class="menu-item" @click="choose('note')">
                <Icon name="note" :size="15" /> Secure note
              </button>
              <button type="button" class="menu-item" @click="choose('card')">
                <Icon name="card" :size="15" /> Card
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </div>

    <div class="rows">
      <button
        v-for="item in items"
        :key="item.id"
        type="button"
        class="row-item"
        :class="{ active: detail?.id === item.id }"
        @click="emit('select', item.id)"
      >
        <span
          class="avatar"
          :style="{ background: `linear-gradient(155deg, hsl(${hueFromText(item.title)} 62% 58%), hsl(${hueFromText(item.title) + 26} 58% 42%))` }"
        >
          {{ initials(item.title) }}
        </span>
        <span class="row-text">
          <span class="row-title truncate">{{ item.title }}</span>
          <span class="row-sub truncate">
            {{ item.username || hostFromUrl(item.url) || (item.type === 'note' ? 'Secure note' : 'Card') }}
          </span>
        </span>
        <span class="row-meta">
          <span v-if="item.hasTOTP" class="pip" title="Has a one-time password">
            <Icon name="clock" :size="13" />
          </span>
          <button
            class="star"
            :class="{ on: item.favorite }"
            type="button"
            :title="item.favorite ? 'Remove from favorites' : 'Add to favorites'"
            @click.stop="toggleFavorite(item.id)"
          >
            <Icon :name="item.favorite ? 'starFilled' : 'star'" :size="14" />
          </button>
          <span class="faint when">{{ relativeTime(item.updatedAt) }}</span>
        </span>
      </button>

      <div v-if="items.length === 0" class="empty">
        <Icon :name="activeFilters ? 'search' : 'shield'" :size="26" />
        <p class="empty-title">{{ activeFilters ? 'No items match' : 'Your vault is empty' }}</p>
        <p class="faint empty-text">
          {{
            activeFilters
              ? 'Try a different search term, or clear the filters.'
              : 'Add your first login, note or card. Everything is encrypted before it touches the disk.'
          }}
        </p>
        <button v-if="activeFilters" class="btn small" type="button" @click="clearQuery(); applyFilters({ type: '', tag: '', favoritesOnly: false })">
          Clear filters
        </button>
        <button v-else class="btn small primary" type="button" @click="choose('login')">
          <Icon name="plus" :size="14" />
          Add an item
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.list-pane {
  width: var(--list);
  flex: none;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  background: var(--surface-2);
  background: color-mix(in srgb, var(--bg) 55%, var(--surface));
  min-height: 0;
}

.toolbar {
  padding: 12px 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 9px;
  border-bottom: 1px solid var(--border);
}

.search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 9px 7px 11px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text-mute);
}

.search:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
  color: var(--text-dim);
}

.search-field {
  flex: 1;
  min-width: 0;
  border: 0;
  background: none;
  outline: none;
  font-size: 13px;
  color: var(--text);
}

.search-field::-webkit-search-cancel-button {
  display: none;
}

.toolbar-row {
  display: flex;
  gap: 6px;
}

.create {
  position: relative;
  display: flex;
}

.create .caret {
  padding-left: 4px;
  padding-right: 5px;
  border-left: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 0 var(--radius-xs) var(--radius-xs) 0;
}

.create > .btn.primary:first-child {
  border-radius: var(--radius-xs) 0 0 var(--radius-xs);
}

.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  z-index: 20;
  padding: 5px;
  min-width: 168px;
  background: var(--surface);
  border: 1px solid var(--border-strong);
  box-shadow: var(--shadow-2);
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 9px;
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
}

.menu-item:hover {
  background: var(--surface-3);
  color: var(--text);
}

.menu-enter-active,
.menu-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}

.menu-enter-from,
.menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.rows {
  flex: 1;
  overflow: auto;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-height: 0;
}

.row-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 7px 8px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: none;
  cursor: pointer;
  text-align: left;
  color: inherit;
  transition: background 0.12s ease, border-color 0.12s ease;
}

.row-item:hover {
  background: var(--surface-2);
}

.row-item.active {
  background: var(--accent-soft);
  border-color: var(--border-strong);
  border-color: color-mix(in srgb, var(--accent) 34%, transparent);
}

.row-item .avatar {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  font-size: 11.5px;
}

.row-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}

.row-title {
  font-size: 13.5px;
  font-weight: 650;
}

.row-sub {
  font-size: 11.5px;
  color: var(--text-mute);
}

.row-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: none;
}

.pip {
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  border-radius: 6px;
  background: var(--accent-soft);
  color: var(--accent-strong);
}

.star {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--text-mute);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease, color 0.12s ease;
}

.row-item:hover .star,
.star.on {
  opacity: 1;
}

.star.on {
  color: var(--warn);
}

.when {
  font-size: 10.5px;
  width: 54px;
  text-align: right;
}

.empty {
  margin: auto;
  padding: 26px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
  color: var(--text-mute);
}

.empty-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-dim);
}

.empty-text {
  font-size: 12.5px;
  line-height: 1.6;
  max-width: 260px;
}
</style>
