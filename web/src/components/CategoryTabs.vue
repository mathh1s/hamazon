<script setup lang="ts">
import ItemIcon from './ItemIcon.vue'
import { hideTooltip, moveTooltip, showTooltip } from '../stores/tooltip'
import { playClick } from '../stores/sound'

export interface Tab {
  id: string
  name: string
  icon: string
}

defineProps<{ tabs: Tab[]; selected: string; position: 'top' | 'bottom' }>()
const emit = defineEmits<{ select: [string] }>()

function hover(e: PointerEvent, tab: Tab) {
  if (e.pointerType === 'mouse') showTooltip(e, tab.name)
}

function pick(id: string) {
  playClick()
  emit('select', id)
}
</script>

<template>
  <div class="tabs" :class="position" role="tablist" :aria-label="position === 'top' ? 'Categories' : 'More categories'">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      role="tab"
      class="tab"
      :class="{ selected: tab.id === selected }"
      :aria-selected="tab.id === selected"
      :aria-label="tab.name"
      @click="pick(tab.id)"
      @pointerenter="hover($event, tab)"
      @pointermove="moveTooltip"
      @pointerleave="hideTooltip"
    >
      <ItemIcon :texture="tab.icon" />
    </button>
  </div>
</template>

<style scoped>
.tabs {
  position: relative;
  display: flex;
  gap: var(--u);
  height: calc(var(--u) * 28);
}

.tabs.top {
  margin-bottom: calc(var(--u) * -4);
  align-items: flex-start;
}

.tabs.bottom {
  margin-top: calc(var(--u) * -4);
  align-items: flex-end;
}

.tab {
  position: relative;
  width: calc(var(--u) * 26);
  height: calc(var(--u) * 32);
  padding: 0;
  border: 0;
  background: url('/mc/gui/tab.png') center / 100% 100%;
  cursor: pointer;
  z-index: 0;
}

.bottom .tab {
  background-image: url('/mc/gui/tab_bottom.png');
}

/* The selected tab is drawn over the panel border so it merges with it. */
.tab.selected {
  background-image: url('/mc/gui/tab_selected.png');
  z-index: 2;
}

.bottom .tab.selected {
  background-image: url('/mc/gui/tab_bottom_selected.png');
}

.tab .icon {
  position: absolute;
  left: calc(var(--u) * 5);
}

.top .tab .icon {
  top: calc(var(--u) * 8);
}

.bottom .tab .icon {
  top: calc(var(--u) * 8);
}

.tab:focus-visible {
  outline-offset: calc(var(--u) * -2);
}
</style>
