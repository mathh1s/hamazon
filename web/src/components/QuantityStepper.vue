<script setup lang="ts">
import { ref, watch } from 'vue'
import McButton from './McButton.vue'

const props = defineProps<{ modelValue: number; min?: number; max: number; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [number] }>()

const draft = ref(String(props.modelValue))
watch(
  () => props.modelValue,
  (v) => (draft.value = String(v)),
)

function clamp(n: number) {
  const min = props.min ?? 1
  return Math.max(min, Math.min(props.max, Number.isFinite(n) ? Math.floor(n) : min))
}

function commit() {
  const next = clamp(Number(draft.value))
  draft.value = String(next)
  if (next !== props.modelValue) emit('update:modelValue', next)
}

function step(delta: number) {
  emit('update:modelValue', clamp(props.modelValue + delta))
}
</script>

<template>
  <span class="stepper" role="group" :aria-label="label">
    <McButton small :disabled="modelValue <= (min ?? 1)" aria-label="Decrease" @click="step(-1)">-</McButton>
    <input
      v-model="draft"
      class="value shadow"
      inputmode="numeric"
      :aria-label="label"
      @blur="commit"
      @keydown.enter.prevent="commit"
      @keydown.up.prevent="step(1)"
      @keydown.down.prevent="step(-1)"
    />
    <McButton small :disabled="modelValue >= max" aria-label="Increase" @click="step(1)">+</McButton>
  </span>
</template>

<style scoped>
.stepper {
  display: inline-flex;
  align-items: center;
  gap: var(--u);
}

.value {
  width: calc(var(--u) * 22);
  height: calc(var(--u) * 16);
  border: var(--u) solid #a0a0a0;
  background: #000;
  color: var(--text);
  text-align: center;
  outline: none;
}

.value:focus {
  border-color: var(--text);
}
</style>
