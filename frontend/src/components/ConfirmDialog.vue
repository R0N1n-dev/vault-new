<script setup lang="ts">
import Modal from './Modal.vue'

withDefaults(
  defineProps<{
    open: boolean
    title: string
    message: string
    detail?: string
    confirmLabel?: string
    cancelLabel?: string
    danger?: boolean
  }>(),
  { confirmLabel: 'Confirm', cancelLabel: 'Cancel', danger: false },
)

const emit = defineEmits<{ (event: 'confirm'): void; (event: 'cancel'): void }>()
</script>

<template>
  <Modal :open="open" :title="title" :width="'440px'" @close="emit('cancel')">
    <p class="message">{{ message }}</p>
    <p v-if="detail" class="faint detail">{{ detail }}</p>
    <template #footer>
      <button class="btn" type="button" @click="emit('cancel')">{{ cancelLabel }}</button>
      <button class="btn" :class="danger ? 'danger' : 'primary'" type="button" @click="emit('confirm')">
        {{ confirmLabel }}
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.message {
  font-size: 14px;
  line-height: 1.6;
}

.detail {
  margin-top: 8px;
  font-size: 12.5px;
}
</style>
