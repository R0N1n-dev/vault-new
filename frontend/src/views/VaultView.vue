<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import ItemDetail from '../components/ItemDetail.vue'
import ItemEditor from '../components/ItemEditor.vue'
import ItemList from '../components/ItemList.vue'
import Sidebar from '../components/Sidebar.vue'
import GeneratorPanel from '../components/GeneratorPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import AuditPanel from '../components/AuditPanel.vue'
import TrashPanel from '../components/TrashPanel.vue'
import ImportPanel from '../components/ImportPanel.vue'
import Icon from '../components/Icon.vue'
import {
  backendKind,
  confirmDelete,
  confirmPurge,
  copy,
  detail,
  editing,
  effectiveSettings,
  generatorOpen,
  importOpen,
  items,
  lock,
  pendingDelete,
  pendingPurge,
  registerActivity,
  resetPreview,
  select,
  settings,
  settingsOpen,
  startCreate,
  startEdit,
  switchView,
  view,
} from '../stores/session'

const listRef = ref<{ focusSearch: () => void; focusCreate: () => void } | null>(null)

const anyModalOpen = computed(
  () =>
    editing.value !== null ||
    settingsOpen.value ||
    generatorOpen.value ||
    importOpen.value ||
    pendingDelete.value !== null ||
    pendingPurge.value !== null,
)

function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable
}

function move(delta: number): void {
  if (items.value.length === 0) return
  const index = items.value.findIndex((item) => item.id === detail.value?.id)
  const next = index < 0 ? (delta > 0 ? 0 : items.value.length - 1) : Math.min(Math.max(index + delta, 0), items.value.length - 1)
  const target = items.value[next]
  if (target) void select(target.id)
}

function onKeydown(event: KeyboardEvent): void {
  const mod = event.metaKey || event.ctrlKey
  const key = event.key.toLowerCase()

  if (mod && key === 'k') {
    event.preventDefault()
    listRef.value?.focusSearch()
    return
  }
  if (mod && key === 'n') {
    event.preventDefault()
    switchView('items')
    startCreate('login')
    return
  }
  if (mod && key === 'l') {
    event.preventDefault()
    void lock()
    return
  }
  if (mod && key === 'g') {
    event.preventDefault()
    generatorOpen.value = true
    return
  }
  if (mod && event.key === ',') {
    event.preventDefault()
    settingsOpen.value = true
    return
  }
  if (key === 'escape' && !anyModalOpen.value) {
    void select(null)
    return
  }
  if (anyModalOpen.value || isTyping(event.target)) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    move(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    move(-1)
    return
  }
  if (key === 'e' && detail.value) {
    event.preventDefault()
    startEdit(detail.value)
  }
}

function onBlur(): void {
  if (settings.value?.lockOnBlur) void lock()
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('blur', onBlur)
  for (const event of ['pointerdown', 'keydown', 'wheel', 'focus'] as const) {
    window.addEventListener(event, registerActivity, { passive: true })
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('blur', onBlur)
  for (const event of ['pointerdown', 'keydown', 'wheel', 'focus'] as const) {
    window.removeEventListener(event, registerActivity)
  }
})
</script>

<template>
  <div class="vault">
    <div v-if="backendKind === 'demo'" class="preview-banner">
      <Icon name="info" :size="15" />
      <span class="grow">
        <strong>Browser preview.</strong>
        This build runs the interface without the Go engine: the vault lives in localStorage and uses
        PBKDF2-SHA256 + AES-256-GCM. The desktop app uses Argon2id + XChaCha20-Poly1305 in a single file.
      </span>
      <button class="btn small quiet" type="button" @click="resetPreview">
        <Icon name="refresh" :size="13" />
        Reset preview
      </button>
    </div>

    <div class="workspace">
      <Sidebar @settings="settingsOpen = true" />
      <template v-if="view === 'items'">
        <ItemList ref="listRef" @select="select" />
        <ItemDetail @edit="startEdit" />
      </template>
      <AuditPanel v-else-if="view === 'audit'" />
      <TrashPanel v-else />
    </div>

    <ItemEditor />
    <ImportPanel />
    <SettingsPanel :open="settingsOpen" @close="settingsOpen = false" />
    <GeneratorPanel
      :open="generatorOpen"
      :options="effectiveSettings.generator"
      title="Password generator"
      accept-label="Copy to clipboard"
      @close="generatorOpen = false"
      @use="(value: string) => copy(value, 'Generated password')"
    />
    <ConfirmDialog
      :open="pendingDelete !== null"
      title="Move this item to the trash?"
      :message="`“${pendingDelete?.title ?? ''}” will leave the vault and stay in the trash until it expires.`"
      detail="You can restore it from the trash, and the trash is only emptied once its retention window closes."
      confirm-label="Move to trash"
      danger
      @cancel="pendingDelete = null"
      @confirm="confirmDelete"
    />
    <ConfirmDialog
      :open="pendingPurge !== null"
      title="Delete this item for good?"
      :message="`“${pendingPurge?.title ?? ''}” will be removed from the vault file.`"
      detail="This cannot be undone — the item is not kept anywhere else."
      confirm-label="Delete forever"
      danger
      @cancel="pendingPurge = null"
      @confirm="confirmPurge"
    />
  </div>
</template>

<style scoped>
.vault {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.preview-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 14px;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--accent-strong);
  background: var(--accent-soft);
  border-bottom: 1px solid var(--border);
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
}

.workspace {
  flex: 1;
  display: flex;
  min-height: 0;
}
</style>
