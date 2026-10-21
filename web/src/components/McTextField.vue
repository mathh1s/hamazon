<script setup lang="ts">
defineOptions({ inheritAttrs: false })
const model = defineModel<string>({ required: true })
defineProps<{ invalid?: boolean }>()
</script>

<template>
  <input v-model="model" class="mc-field shadow" :class="{ invalid }" v-bind="$attrs" />
</template>

<style scoped>
.mc-field {
  display: block;
  width: 100%;
  min-height: calc(var(--u) * 20);
  padding: 0 calc(var(--u) * 4);
  border: var(--u) solid transparent;
  border-image: url('/mc/gui/text_field.png') 1 fill / var(--u) stretch;
  background: none;
  color: #e0e0e0;
  caret-color: var(--text);
  outline: none;
}

.mc-field::placeholder {
  color: #707070;
  text-shadow: none;
}

.mc-field:focus {
  border-image-source: url('/mc/gui/text_field_highlight.png');
  color: var(--text);
}

.mc-field.invalid {
  border-image-source: none;
  border-color: var(--red);
  background: #000;
}
</style>
