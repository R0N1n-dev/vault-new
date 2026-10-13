import { ref } from 'vue'
import type { VaultEntry } from '../lib/types'
import { Unlock, Lock, GetEntries, AddEntry, UpdateEntry, DeleteEntry } from '../../wailsjs/go'

export function useVault() {
  const entries = ref<VaultEntry[]>([])
  const locked = ref(true)

  async function loadEntries() {
    try {
      entries.value = await GetEntries()
    } catch (e) {
      console.error('failed to load entries', e)
    }
  }

  async function addEntry(name: string, value: string, kind: string) {
    await AddEntry(name, value, kind)
    await loadEntries()
  }

  async function updateEntry(id: string, name: string, value: string, kind: string) {
    await UpdateEntry(id, name, value, kind)
    await loadEntries()
  }

  async function deleteEntry(id: string) {
    await DeleteEntry(id)
    await loadEntries()
  }

  async function doUnlock(pin: string) {
    await Unlock(pin)
    locked.value = false
    await loadEntries()
  }

  async function doLock() {
    await Lock()
    locked.value = true
    entries.value = []
  }

  return { entries, locked, loadEntries, addEntry, updateEntry, deleteEntry, doUnlock, doLock }
}
