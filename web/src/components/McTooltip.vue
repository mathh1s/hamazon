<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { tooltip } from '../stores/tooltip'

const el = ref<HTMLElement | null>(null)
const pos = ref({ left: 0, top: 0 })

// Like the game: offset from the cursor, flipped to stay on screen.
watch(
  () => [tooltip.x, tooltip.y, tooltip.visible, tooltip.title],
  async () => {
    if (!tooltip.visible) return
    await nextTick()
    const box = el.value?.getBoundingClientRect()
    const w = box?.width ?? 0
    const h = box?.height ?? 0
    let left = tooltip.x + 16
    let top = tooltip.y - 16
    if (left + w > window.innerWidth - 4) left = tooltip.x - w - 16
    if (top + h > window.innerHeight - 4) top = window.innerHeight - h - 4
    pos.value = { left: Math.max(4, left), top: Math.max(4, top) }
  },
)

const style = computed(() => ({ left: pos.value.left + 'px', top: pos.value.top + 'px' }))
</script>

<template>
  <div v-show="tooltip.visible" ref="el" class="tooltip shadow" :style="style" aria-hidden="true">
    <div class="title">{{ tooltip.title }}</div>
    <div v-for="(line, i) in tooltip.lines" :key="i" class="line" :style="{ color: line.color ?? 'var(--gray)' }">
      {{ line.text }}
    </div>
  </div>
</template>

<style scoped>
.tooltip {
  position: fixed;
  z-index: 100;
  max-width: min(90vw, calc(var(--u) * 160));
  padding: calc(var(--u) * 2) calc(var(--u) * 3);
  background: var(--tooltip-bg);
  border: var(--u) solid;
  border-image: linear-gradient(rgba(80, 0, 255, 0.8), rgba(40, 0, 127, 0.8)) 1;
  box-shadow: 0 0 0 var(--u) var(--tooltip-bg);
  pointer-events: none;
}

.title {
  color: var(--text);
  margin-bottom: calc(var(--u) * 2);
}

.title:only-child {
  margin-bottom: 0;
}
</style>
