<script setup lang="ts">
import { computed, ref } from 'vue'
import { closeImport, commitImport, importBusy, importOpen, importPreview, previewImport } from '../stores/session'
import Icon from './Icon.vue'
import Modal from './Modal.vue'

const fileName = ref('')
const reading = ref(false)
const readError = ref('')

const preview = computed(() => importPreview.value)
const importable = computed(() => preview.value?.new ?? 0)
const totalRows = computed(() => preview.value?.rows.length ?? 0)
const shown = computed(() => (preview.value?.rows ?? []).slice(0, 120))
const hidden = computed(() => Math.max(totalRows.value - shown.value.length, 0))

/**
 * The file is read in the webview with the File API, so the same code path
 * works in the packaged app and in the browser preview. The text is handed to
 * the vault, which does the parsing.
 */
async function readFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  readError.value = ''
  reading.value = true
  fileName.value = file.name
  try {
    await previewImport(await file.text())
  } catch (error) {
    readError.value = error instanceof Error ? error.message : 'That file could not be read.'
  } finally {
    reading.value = false
    input.value = ''
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'duplicate':
      return 'Already here'
    case 'invalid':
      return 'Skipped'
    case 'unsupported':
      return 'Not supported'
    default:
      return 'New'
  }
}
</script>

<template>
  <Modal
    :open="importOpen"
    title="Import from CSV"
    subtitle="Move passwords in from Bitwarden, 1Password, KeePass, Chrome or any spreadsheet."
    width="720px"
    @close="closeImport"
  >
    <div class="stack">
      <div class="drop">
        <Icon name="upload" :size="20" />
        <div class="grow">
          <div class="drop-title">Choose a CSV export</div>
          <p class="faint drop-note">
            The file is read here, parsed in the vault, and never copied into the vault folder. Delete the
            original export once the import is done — a CSV is plain text.
          </p>
        </div>
        <label class="btn primary file-btn">
          <input type="file" accept=".csv,text/csv,text/plain" :disabled="reading || importBusy" @change="readFile" />
          Browse…
        </label>
      </div>

      <p v-if="reading" class="faint">Reading {{ fileName }}…</p>
      <p v-if="readError" class="error-text">
        <Icon name="alert" :size="13" />
        {{ readError }}
      </p>

      <template v-if="preview">
        <div class="summary">
          <div class="stat">
            <span class="stat-value">{{ preview.format }}</span>
            <span class="faint stat-label">Detected format</span>
          </div>
          <div class="stat">
            <span class="stat-value">{{ importable }}</span>
            <span class="faint stat-label">Ready to import</span>
          </div>
          <div class="stat" :class="{ mute: preview.duplicates === 0 }">
            <span class="stat-value">{{ preview.duplicates }}</span>
            <span class="faint stat-label">Already in the vault</span>
          </div>
          <div class="stat" :class="{ mute: preview.invalid + preview.unsupported === 0 }">
            <span class="stat-value">{{ preview.invalid + preview.unsupported }}</span>
            <span class="faint stat-label">Cannot be imported</span>
          </div>
        </div>

        <ul v-if="preview.warnings && preview.warnings.length > 0" class="warnings">
          <li v-for="(warning, index) in preview.warnings.slice(0, 6)" :key="index">
            <Icon name="alert" :size="12" />
            <span>{{ warning }}</span>
          </li>
          <li v-if="preview.warnings.length > 6" class="faint">
            and {{ preview.warnings.length - 6 }} more warnings
          </li>
        </ul>

        <div class="table">
          <div class="table-head">
            <span class="col-status">Status</span>
            <span class="col-title">Item</span>
            <span class="col-user">Username</span>
            <span class="col-reason">Note</span>
          </div>
          <div v-for="row in shown" :key="row.line" class="table-row" :class="row.status">
            <span class="col-status">{{ statusLabel(row.status) }}</span>
            <span class="col-title truncate">{{ row.title }}</span>
            <span class="col-user faint truncate">{{ row.username }}</span>
            <span class="col-reason faint truncate">{{ row.reason ?? '' }}</span>
          </div>
          <div v-if="hidden > 0" class="table-row faint">
            <span class="col-status">…</span>
            <span class="col-title">and {{ hidden }} more rows</span>
          </div>
        </div>

        <p class="faint hint-line">
          Imported items land in the vault as normal entries, tagged with their folder and keeping their
          one-time password secrets where the export included them.
        </p>
      </template>
    </div>

    <template #footer>
      <button class="btn" type="button" @click="closeImport">Cancel</button>
      <button
        v-if="preview && preview.duplicates > 0"
        class="btn quiet"
        type="button"
        :disabled="importBusy || totalRows === 0"
        @click="commitImport('add')"
      >
        Import all {{ totalRows }} rows
      </button>
      <button
        class="btn primary"
        type="button"
        :disabled="importBusy || importable === 0"
        @click="commitImport('skip')"
      >
        Import {{ importable }} {{ importable === 1 ? 'item' : 'items' }}
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.drop {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius);
  background: var(--surface-2);
  color: var(--accent);
}

.drop-title {
  font-size: 13.5px;
  font-weight: 700;
  color: var(--text);
}

.drop-note {
  font-size: 11.5px;
  line-height: 1.55;
  max-width: 52ch;
  margin-top: 2px;
}

.file-btn {
  position: relative;
  overflow: hidden;
  flex: none;
}

.file-btn input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
}

.summary {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

.stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface);
}

.stat.mute {
  opacity: 0.6;
}

.stat-value {
  font-size: 15px;
  font-weight: 700;
}

.stat-label {
  font-size: 11px;
}

.warnings {
  list-style: none;
  margin: 0;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 5px;
  border-radius: var(--radius-sm);
  background: var(--warn-soft);
  color: var(--warn);
  font-size: 11.5px;
  line-height: 1.5;
}

.warnings li {
  display: flex;
  align-items: flex-start;
  gap: 7px;
}

.table {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
  max-height: 262px;
  overflow-y: auto;
}

.table-head,
.table-row {
  display: grid;
  grid-template-columns: 104px minmax(120px, 1.2fr) minmax(100px, 1fr) minmax(120px, 1.4fr);
  gap: 10px;
  align-items: center;
  padding: 7px 11px;
  font-size: 12px;
}

.table-head {
  position: sticky;
  top: 0;
  background: var(--surface-3);
  color: var(--text-mute);
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.table-row {
  border-top: 1px solid var(--border);
  background: var(--surface);
}

.table-row.duplicate,
.table-row.invalid,
.table-row.unsupported {
  color: var(--text-mute);
}

.table-row.duplicate .col-status {
  color: var(--warn);
}

.table-row.invalid .col-status,
.table-row.unsupported .col-status {
  color: var(--bad);
}

.col-status {
  font-size: 11px;
  font-weight: 700;
}

.hint-line {
  font-size: 11.5px;
  line-height: 1.55;
}
</style>
