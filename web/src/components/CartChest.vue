<script setup lang="ts">
import { computed } from 'vue'
import ItemSlot from './ItemSlot.vue'
import PriceTag from './PriceTag.vue'
import McButton from './McButton.vue'
import { cartLines, cartTotals } from '../stores/cart'
import { hideTooltip, moveTooltip, showTooltip } from '../stores/tooltip'

const SLOTS = 27
const emit = defineEmits<{ select: [number] }>()

// Like a real chest, every purchase unit takes its own slot.
const slots = computed(() => {
  const out: { itemId: number; texture: string; count: number; name: string; quantity: number; inStock: boolean }[] = []
  for (const line of cartLines.value) {
    if (!line.item) continue
    for (let i = 0; i < line.entry.quantity; i++) {
      out.push({
        itemId: line.item.id,
        texture: line.item.texture,
        count: line.item.count,
        name: line.item.name,
        quantity: line.entry.quantity,
        inStock: line.item.inStock,
      })
    }
  }
  return out
})
const overflow = computed(() => Math.max(0, slots.value.length - SLOTS))
const visible = computed(() => (overflow.value > 0 ? slots.value.slice(0, SLOTS - 1) : slots.value))
const empty = computed(() => SLOTS - visible.value.length - (overflow.value > 0 ? 1 : 0))
const hasItems = computed(() => slots.value.length > 0)

function hover(e: PointerEvent, s: { name: string; quantity: number; count: number }) {
  if (e.pointerType === 'mouse') showTooltip(e, s.name, [{ text: `${s.quantity} × ${s.count} in cart` }])
}
</script>

<template>
  <section class="mc-panel chest" aria-labelledby="chest-title">
    <h2 id="chest-title" class="mc-title">Cart</h2>
    <div class="grid">
      <ItemSlot
        v-for="(s, i) in visible"
        :key="i"
        :texture="s.texture"
        :count="s.count"
        interactive
        :dimmed="!s.inStock"
        :label="`${s.name}, ${s.quantity} in cart`"
        @click="emit('select', s.itemId)"
        @pointerenter="hover($event, s)"
        @pointermove="moveTooltip"
        @pointerleave="hideTooltip"
      />
      <span v-if="overflow > 0" class="more" :aria-label="`${overflow + 1} more`">+{{ overflow + 1 }}</span>
      <ItemSlot v-for="i in empty" :key="'e' + i" />
    </div>
    <div v-if="hasItems" class="summary">
      <span>Total</span>
      <PriceTag :price="cartTotals" />
    </div>
    <p v-else class="hint">Your cart is empty. Pick an item and add it here.</p>
    <McButton to="/cart" :disabled="!hasItems">Review cart</McButton>
  </section>
</template>

<style scoped>
.chest {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 3);
  width: fit-content;
  max-width: 100%;
}

.grid {
  display: grid;
  grid-template-columns: repeat(9, calc(var(--u) * 18));
}

.more {
  display: flex;
  align-items: center;
  justify-content: center;
  width: calc(var(--u) * 18);
  height: calc(var(--u) * 18);
  background: url('/mc/gui/slot.png') center / 100% 100%;
  color: var(--text);
  text-shadow: var(--u) var(--u) 0 var(--text-shadow);
}

.summary {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: calc(var(--u) * 2);
}

.hint {
  margin: 0;
  color: var(--dark-gray);
  max-width: calc(var(--u) * 162);
}
</style>
