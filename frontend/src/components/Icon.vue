<script setup lang="ts">
import { computed } from 'vue'
import { iconPaths, type IconName } from '../lib/icons'

const props = withDefaults(
  defineProps<{
    name: IconName
    size?: number | string
    strokeWidth?: number
  }>(),
  { size: 18, strokeWidth: 1.7 },
)

const definition = computed(() => iconPaths(props.name))
const dimension = computed(() => (typeof props.size === 'number' ? `${props.size}px` : props.size))
</script>

<template>
  <svg
    class="icon"
    :width="dimension"
    :height="dimension"
    viewBox="0 0 24 24"
    :fill="definition.filled ? 'currentColor' : 'none'"
    stroke="currentColor"
    :stroke-width="definition.filled ? 0 : strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path v-for="(path, index) in definition.paths" :key="index" :d="path" />
  </svg>
</template>

<style scoped>
.icon {
  display: block;
  flex: none;
  overflow: visible;
}
</style>
