<script setup lang="ts">
import ItemIcon from './ItemIcon.vue'

withDefaults(
  defineProps<{
    texture?: string
    count?: number
    selected?: boolean
    interactive?: boolean
    label?: string
    size?: number
    dimmed?: boolean
  }>(),
  { size: 18 },
)
defineEmits<{ click: [MouseEvent] }>()
</script>

<template>
  <component
    :is="interactive ? 'button' : 'span'"
    class="slot"
    :class="{ selected, interactive, dimmed }"
    :style="{ '--slot': size }"
    :type="interactive ? 'button' : undefined"
    :aria-label="label"
    :aria-pressed="interactive ? selected : undefined"
    @click="$emit('click', $event)"
  >
    <ItemIcon v-if="texture" :texture="texture" :size="size - 2" />
    <span v-if="count && count > 1" class="count shadow" aria-hidden="true">{{ count }}</span>
  </component>
</template>

<style scoped>
.slot {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: calc(var(--u) * var(--slot));
  height: calc(var(--u) * var(--slot));
  padding: 0;
  border: 0;
  background: url('/mc/gui/slot.png') center / 100% 100%;
  color: var(--text);
  flex: none;
}

.slot.interactive {
  cursor: pointer;
}

/* The game whitens the slot under the cursor. */
.slot.interactive:hover::after,
.slot.interactive:focus-visible::after,
.slot.selected::after {
  content: '';
  position: absolute;
  inset: var(--u);
  background: rgba(255, 255, 255, 0.5);
  pointer-events: none;
}

.slot.selected {
  outline: var(--u) solid var(--text);
  outline-offset: calc(var(--u) * -1);
}

.slot:focus-visible {
  outline: var(--u) solid var(--yellow);
  outline-offset: 0;
}

/* Like a locked slot: the item is greyed out and darkened. */
.slot.dimmed :deep(.icon) {
  filter: grayscale(1) brightness(0.55);
}

.slot.dimmed .count {
  color: var(--gray);
}

.count {
  position: absolute;
  right: 0;
  bottom: calc(var(--u) * -1);
  z-index: 1;
  line-height: 1;
  pointer-events: none;
}
</style>
