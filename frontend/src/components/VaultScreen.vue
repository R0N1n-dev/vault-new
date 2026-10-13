<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { VaultEntry } from '../lib/types'
import { useVault } from '../composables/useVault'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const { entries, locked, loadEntries, addEntry, updateEntry, deleteEntry, doLock } = useVault()
const name = ref('')
const value = ref('')
const kind = ref('password')
const editing = ref<VaultEntry | null>(null)
const showAdd = ref(false)

let cleanup: (() => void) | undefined
onMounted(() => {
  loadEntries()
  cleanup = EventsOn('vault:lock', () => {
    locked.value = true
    entries.value = []
  })
  cleanup = EventsOn('menu:new-entry', () => {
    startAdd()
  })
})
onUnmounted(() => {
  cleanup?.()
})

function startAdd() {
  editing.value = null
  name.value = ''
  value.value = ''
  kind.value = 'password'
  showAdd.value = true
}

async function save() {
  if (!name.value || !value.value) return
  if (editing.value) {
    await updateEntry(editing.value.id, name.value, value.value, kind.value)
  } else {
    await addEntry(name.value, value.value, kind.value)
  }
  showAdd.value = false
}

function startEdit(e: VaultEntry) {
  editing.value = e
  name.value = e.name
  value.value = e.value
  kind.value = e.kind
  showAdd.value = true
}

async function remove(e: VaultEntry) {
  await deleteEntry(e.id)
}
</script>

<template>
  <div class="vault">
    <header>
      <h1>Vault</h1>
      <button @click="doLock">Lock</button>
    </header>

    <main>
      <button class="add" @click="startAdd">+ Add Entry</button>

      <div v-if="showAdd" class="editor">
        <input v-model="name" placeholder="Name" />
        <input v-model="value" type="password" placeholder="Value" />
        <select v-model="kind">
          <option value="password">Password</option>
          <option value="note">Note</option>
          <option value="card">Card</option>
        </select>
        <button @click="save">Save</button>
      </div>

      <ul v-if="entries.length">
        <li v-for="e in entries" :key="e.id">
          <span class="name">{{ e.name }}</span>
          <span class="kind">{{ e.kind }}</span>
          <button @click="startEdit(e)">Edit</button>
          <button @click="remove(e)">Delete</button>
        </li>
      </ul>
    </main>
  </div>
</template>

<style scoped>
.vault {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
header {
  display: flex;
  justify-content: space-between;
  padding: 1rem;
  border-bottom: 1px solid #333;
}
main {
  flex: 1;
  padding: 1rem;
  overflow: auto;
}
.add {
  width: 100%;
  padding: 0.5rem;
  margin-bottom: 1rem;
}
.editor {
  border: 1px solid #444;
  padding: 1rem;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
input, select {
  padding: 0.5rem;
}
ul {
  list-style: none;
  padding: 0;
}
li {
  display: flex;
  gap: 1rem;
  padding: 0.5rem;
  border-bottom: 1px solid #222;
  align-items: center;
}
.name { flex: 1; }
.kind { color: #888; font-size: 0.8rem; }
button {
  padding: 0.3rem 0.6rem;
}
</style>
