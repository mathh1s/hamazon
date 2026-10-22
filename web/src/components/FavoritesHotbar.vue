<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import ItemIcon from './ItemIcon.vue'
import type { Item } from '../api'
import { itemsById } from '../stores/catalog'
import { favorites, HOTBAR_SIZE } from '../stores/favorites'
import { hideTooltip, moveTooltip, showTooltip } from '../stores/tooltip'

const props = defineProps<{ selectedId: number | null }>()
const emit = defineEmits<{ select: [Item] }>()

const slots = computed(() =>
  Array.from({ length: HOTBAR_SIZE }, (_, i) => {
    const id = favorites.ids[i]
    return id === undefined ? undefined : itemsById.value.get(id)
  }),
)
const selectedIndex = computed(() => slots.value.findIndex((s) => s && s.id === props.selectedId))

// Number keys pick a hotbar slot, like in game, unless the player is typing.
function onKey(e: KeyboardEvent) {
  if (e.altKey || e.ctrlKey || e.metaKey) return
  const target = e.target as HTMLElement | null
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return
  const n = Number(e.key)
  if (!Number.isInteger(n) || n < 1 || n > HOTBAR_SIZE) return
  const item = slots.value[n - 1]
  if (item) emit('select', item)
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

function hover(e: PointerEvent, item: Item) {
  if (e.pointerType === 'mouse') showTooltip(e, item.name)
}
</script>

<template>
  <section class="hotbar-wrap" aria-label="Pinned items">
    <div class="hotbar">
      <span
        v-if="selectedIndex >= 0"
        class="selection"
        :style="{ left: `calc(var(--u) * ${selectedIndex * 20 - 1})` }"
        aria-hidden="true"
      />
      <template v-for="(item, i) in slots" :key="i">
        <button
          v-if="item"
          type="button"
          class="slot"
          :style="{ left: `calc(var(--u) * ${3 + i * 20})` }"
          :aria-label="`${i + 1}: ${item.name}`"
          :class="{ dimmed: !item.inStock }"
          @click="emit('select', item)"
          @pointerenter="hover($event, item)"
          @pointermove="moveTooltip"
          @pointerleave="hideTooltip"
        >
          <ItemIcon :texture="item.texture" />
        </button>
      </template>
    </div>
    <p v-if="favorites.ids.length === 0" class="hint shadow">Pin items from their details to keep them here.</p>
  </section>
</template>

<style scoped>
.hotbar-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--u) * 2);
  margin-top: calc(var(--u) * 6);
}

/* The game's hotbar: 182 by 22 pixels, slots every 20 pixels. */
.hotbar {
  position: relative;
  width: calc(var(--u) * 182);
  height: calc(var(--u) * 22);
  max-width: 100%;
  background: url('/mc/gui/hotbar.png') center / 100% 100%;
}

.selection {
  position: absolute;
  top: calc(var(--u) * -1);
  width: calc(var(--u) * 24);
  height: calc(var(--u) * 23);
  background: url('/mc/gui/hotbar_selection.png') center / 100% 100%;
  pointer-events: none;
  z-index: 1;
}

.slot {
  position: absolute;
  top: calc(var(--u) * 3);
  width: calc(var(--u) * 16);
  height: calc(var(--u) * 16);
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
}

.slot:hover::after {
  content: '';
  position: absolute;
  inset: 0;
  background: rgba(255, 255, 255, 0.4);
}

.slot.dimmed :deep(.icon) {
  filter: grayscale(1) brightness(0.55);
}

.hint {
  margin: 0;
  color: var(--gray);
  text-align: center;
}
</style>
