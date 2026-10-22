<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import { playClick } from '../stores/sound'

const props = withDefaults(
  defineProps<{ to?: RouteLocationRaw; disabled?: boolean; type?: 'button' | 'submit'; active?: boolean; small?: boolean }>(),
  { type: 'button' },
)
const emit = defineEmits<{ click: [MouseEvent] }>()

const tag = computed(() => (props.to && !props.disabled ? RouterLink : 'button'))

function onClick(e: MouseEvent) {
  if (props.disabled) return
  playClick()
  emit('click', e)
}
</script>

<template>
  <component
    :is="tag"
    class="mc-button shadow"
    :class="{ active, small }"
    :to="to && !disabled ? to : undefined"
    :type="tag === 'button' ? type : undefined"
    :disabled="tag === 'button' ? disabled : undefined"
    :aria-current="active ? 'page' : undefined"
    @click="onClick"
  >
    <slot />
  </component>
</template>

<style scoped>
.mc-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: calc(var(--u) * 2);
  min-height: calc(var(--u) * 20);
  padding: 0 calc(var(--u) * 8);
  border: calc(var(--u) * 3) solid transparent;
  border-image: url('/mc/gui/button.png') 3 fill / calc(var(--u) * 3) stretch;
  /* Plain fallback while the sprites are still downloading. */
  border-color: #000;
  background: #6f6f6f padding-box;
  color: var(--text);
  text-decoration: none;
  cursor: pointer;
  white-space: nowrap;
  user-select: none;
}

.mc-button.small {
  min-height: calc(var(--u) * 16);
  padding: 0 calc(var(--u) * 4);
}

.mc-button:hover,
.mc-button:focus-visible,
.mc-button.active {
  border-image-source: url('/mc/gui/button_highlighted.png');
  color: var(--yellow);
}

.mc-button:focus-visible {
  outline: none;
}

.mc-button:disabled {
  border-image-source: url('/mc/gui/button_disabled.png');
  color: #a0a0a0;
  text-shadow: none;
  cursor: default;
}
</style>
