<script setup lang="ts">
import { ref } from 'vue'
import Icon from './Icon.vue'

const props = defineProps<{ modelValue: string[]; placeholder?: string }>()
const emit = defineEmits<{ (event: 'update:modelValue', value: string[]): void }>()

const draft = ref('')

function commit(raw: string): void {
  const values = raw
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
  if (values.length === 0) return
  const next = [...props.modelValue]
  for (const value of values) {
    if (!next.some((tag) => tag.toLowerCase() === value.toLowerCase())) next.push(value)
  }
  emit('update:modelValue', next.sort((a, b) => a.localeCompare(b)))
  draft.value = ''
}

function remove(tag: string): void {
  emit(
    'update:modelValue',
    props.modelValue.filter((candidate) => candidate !== tag),
  )
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter' || event.key === ',') {
    event.preventDefault()
    commit(draft.value)
    return
  }
  if (event.key === 'Backspace' && draft.value === '' && props.modelValue.length > 0) {
    const last = props.modelValue[props.modelValue.length - 1]
    if (last !== undefined) remove(last)
  }
}
</script>

<template>
  <div class="tag-input">
    <span v-for="tag in modelValue" :key="tag" class="chip">
      {{ tag }}
      <button type="button" class="chip-remove" :title="`Remove ${tag}`" @click="remove(tag)">
        <Icon name="close" :size="11" />
      </button>
    </span>
    <input
      class="tag-field"
      :value="draft"
      :placeholder="modelValue.length === 0 ? (placeholder ?? 'Add a tag and press Enter') : ''"
      spellcheck="false"
      @input="draft = ($event.target as HTMLInputElement).value"
      @keydown="onKeydown"
      @blur="commit(draft)"
    />
  </div>
</template>

<style scoped>
.tag-input {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

.tag-input:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.chip-remove {
  display: grid;
  place-items: center;
  border: 0;
  background: none;
  padding: 0;
  color: inherit;
  cursor: pointer;
  opacity: 0.65;
}

.chip-remove:hover {
  opacity: 1;
}

.tag-field {
  flex: 1;
  min-width: 110px;
  border: 0;
  background: none;
  outline: none;
  padding: 2px 0;
  font-size: 13px;
}
</style>
