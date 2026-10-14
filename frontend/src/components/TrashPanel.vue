<script setup lang="ts">
import { computed } from 'vue'
import { formatDateTime, pluralize } from '../lib/format'
import {
  effectiveSettings,
  emptyTrash,
  refreshTrash,
  requestPurge,
  restoreItem,
  switchView,
  trash,
  trashLoading,
} from '../stores/session'
import Icon from './Icon.vue'

const retention = computed(() => effectiveSettings.value.trashRetentionDays)
const empty = computed(() => !trashLoading.value && trash.value.length === 0)

const retentionNote = computed(() =>
  retention.value > 0
    ? `Items stay recoverable for ${retention.value} days, then they are removed from the file the next time the vault is saved.`
    : 'Deleted items are kept until you remove them by hand. The trash is never purged automatically.',
)

/** expiresIn renders how long an item has left before the sweep. */
function expiresIn(deletedAt?: string): string {
  if (!deletedAt) return ''
  const days = retention.value
  if (days <= 0) return 'Kept until removed'
  const left = days - Math.floor((Date.now() - Date.parse(deletedAt)) / 86_400_000)
  if (left <= 0) return 'Removed on the next save'
  if (left === 1) return '1 day left'
  return `${left} days left`
}
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <div class="grow">
        <div class="eyebrow">Trash</div>
        <h2 class="title">Deleted items</h2>
        <p class="faint lede">{{ retentionNote }}</p>
      </div>
      <div class="head-actions">
        <button class="btn quiet" type="button" :disabled="trashLoading" @click="refreshTrash">
          <Icon name="refresh" :size="14" />
          Refresh
        </button>
        <button class="btn quiet danger" type="button" :disabled="trash.length === 0" @click="emptyTrash">
          <Icon name="trash" :size="14" />
          Empty trash
        </button>
        <button class="btn quiet" type="button" @click="switchView('items')">
          <Icon name="chevronLeft" :size="14" />
          Items
        </button>
      </div>
    </header>

    <div v-if="empty" class="empty">
      <Icon name="trash" :size="32" />
      <p>The trash is empty. Deleting an item moves it here instead of removing it straight away.</p>
    </div>

    <ul v-else class="rows">
      <li v-for="item in trash" :key="item.id" class="row-item">
        <div class="mark">
          <Icon :name="item.type === 'card' ? 'card' : item.type === 'note' ? 'note' : 'key'" :size="15" />
        </div>
        <div class="grow">
          <div class="row-title truncate">{{ item.title }}</div>
          <div class="faint row-meta">
            <span v-if="item.username" class="truncate">{{ item.username }}</span>
            <span>Deleted {{ formatDateTime(item.deletedAt) }}</span>
            <span>{{ expiresIn(item.deletedAt) }}</span>
          </div>
        </div>
        <div class="row-actions">
          <button class="btn small quiet" type="button" @click="restoreItem(item.id)">
            <Icon name="history" :size="13" />
            Restore
          </button>
          <button
            class="btn small quiet danger"
            type="button"
            @click="requestPurge({ id: item.id, title: item.title })"
          >
            Delete forever
          </button>
        </div>
      </li>
    </ul>

    <p v-if="!empty" class="faint footnote">
      {{ pluralize(trash.length, 'item') }} in the trash. Nothing here is visible in search, tags or the
      password generator.
    </p>
  </section>
</template>

<style scoped>
.panel {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 20px 26px 32px;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 18px;
}

.head-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.title {
  font-size: 19px;
  font-weight: 700;
  margin: 2px 0 4px;
}

.lede {
  font-size: 12.5px;
  line-height: 1.6;
  max-width: 62ch;
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 40px 20px;
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius);
  color: var(--text-mute);
  text-align: center;
  font-size: 13px;
}

.rows {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.row-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface);
}

.row-item:hover {
  background: var(--surface-2);
}

.mark {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  flex: none;
  border-radius: 8px;
  background: var(--surface-3);
  color: var(--text-dim);
}

.row-title {
  font-size: 13px;
  font-weight: 600;
}

.row-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 10px;
  font-size: 11.5px;
}

.row-actions {
  display: flex;
  gap: 6px;
  flex: none;
}

.footnote {
  margin-top: 16px;
  font-size: 11.5px;
}
</style>
