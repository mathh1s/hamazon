<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import SiteFrame from '../components/SiteFrame.vue'
import ItemSlot from '../components/ItemSlot.vue'
import PriceTag from '../components/PriceTag.vue'
import McButton from '../components/McButton.vue'
import QuantityStepper from '../components/QuantityStepper.vue'
import { catalog, loadCatalog } from '../stores/catalog'
import {
  acceptCurrentPrices,
  cart,
  cartLines,
  cartProblems,
  cartTotals,
  removeFromCart,
  removeOutOfStock,
  setQuantity,
} from '../stores/cart'
import { scalePrice } from '../lib/price'
import { usePageTitle } from '../lib/title'
import { playChestClose, playChestOpen, playPop } from '../stores/sound'

usePageTitle(() => 'Cart')
onMounted(() => {
  playChestOpen()
  loadCatalog(true)
})
onUnmounted(playChestClose)

function remove(id: number) {
  playPop()
  removeFromCart(id)
}

const canCheckout = computed(() => catalog.loaded && cart.entries.length > 0 && cartProblems.value.length === 0)
</script>

<template>
  <SiteFrame>
    <section class="mc-panel" aria-labelledby="cart-title">
      <h2 id="cart-title" class="mc-title">Cart</h2>

      <p v-if="!catalog.loaded && catalog.loading" class="note">Checking prices...</p>
      <p v-else-if="catalog.error" class="note error">{{ catalog.error }}</p>

      <div v-if="cart.entries.length === 0" class="empty">
        <p class="note">Your cart is empty.</p>
        <McButton to="/">Browse the shop</McButton>
      </div>

      <template v-else>
        <div v-if="cartProblems.length" class="problem" role="alert">
          <p v-if="cartProblems.includes('price')">Some prices changed since you added the items. Check the marked lines.</p>
          <p v-if="cartProblems.includes('unavailable')">Some items are no longer sold and will be removed.</p>
          <p v-if="cartProblems.includes('limit')">Some quantities are above the current limit and will be lowered.</p>
          <McButton v-if="cartProblems.some((p) => p !== 'stock')" @click="acceptCurrentPrices">Accept changes</McButton>
          <p v-if="cartProblems.includes('stock')">Some items are out of stock right now.</p>
          <McButton v-if="cartProblems.includes('stock')" @click="removeOutOfStock">Remove out of stock items</McButton>
        </div>

        <ul class="lines">
          <li v-for="line in cartLines" :key="line.entry.itemId" class="line" :class="{ flagged: line.priceChanged || !line.item || line.overLimit || line.outOfStock }">
            <ItemSlot :texture="line.item?.texture" :count="line.item?.count" :dimmed="line.outOfStock" />
            <div class="name">
              <span>{{ line.item?.name ?? 'No longer sold' }}</span>
              <span v-if="line.item" class="unit">
                <PriceTag :price="line.entry.acceptedPrice" /> per {{ line.item.count === 1 ? 'item' : `set of ${line.item.count}` }}
              </span>
              <span v-if="line.outOfStock" class="unit changed">Out of stock</span>
              <span v-if="line.priceChanged && line.item" class="unit changed">
                Now <PriceTag :price="line.item.price" />
              </span>
            </div>
            <QuantityStepper
              v-if="line.item"
              :model-value="line.entry.quantity"
              :max="line.item.maxPerOrder"
              :label="`Quantity of ${line.item.name}`"
              @update:model-value="(n) => setQuantity(line.entry.itemId, n)"
            />
            <PriceTag v-if="line.item" class="total" :price="scalePrice(line.entry.acceptedPrice, line.entry.quantity)" />
            <McButton small :aria-label="`Remove ${line.item?.name ?? 'item'}`" @click="remove(line.entry.itemId)">Remove</McButton>
          </li>
        </ul>

        <div class="footer">
          <div class="sum">
            <span>Total</span>
            <PriceTag :price="cartTotals" />
          </div>
          <div class="actions">
            <McButton to="/">Keep shopping</McButton>
            <McButton to="/checkout" :disabled="!canCheckout">Checkout</McButton>
          </div>
        </div>
      </template>
    </section>
  </SiteFrame>
</template>

<style scoped>
.mc-panel {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 4);
}

.note {
  margin: 0;
  color: var(--dark-gray);
}

.error {
  color: #aa0000;
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: calc(var(--u) * 4);
}

.problem {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: calc(var(--u) * 2);
  padding: calc(var(--u) * 3);
  background: var(--slot-dark);
  color: var(--gold);
  text-shadow: var(--u) var(--u) 0 #3f2a00;
}

.problem p {
  margin: 0;
}

.lines {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.line {
  display: grid;
  grid-template-columns: auto minmax(calc(var(--u) * 80), 1fr) auto auto auto;
  align-items: center;
  gap: calc(var(--u) * 4);
  padding: calc(var(--u) * 2);
  background: #b5b5b5;
  box-shadow: inset var(--u) var(--u) 0 var(--panel-dark), inset calc(var(--u) * -1) calc(var(--u) * -1) 0 var(--panel-light);
}

.line.flagged {
  background: #d8c49a;
}

.name {
  display: flex;
  flex-direction: column;
  gap: var(--u);
  min-width: 0;
}

.unit {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--u);
  color: var(--dark-gray);
}

.changed {
  color: #aa0000;
}

.total {
  min-width: calc(var(--u) * 30);
  justify-content: flex-end;
}

.footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--u) * 4);
}

.sum {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 4);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: calc(var(--u) * 2);
}

@media (max-width: 899px) {
  .line {
    grid-template-columns: auto 1fr auto;
  }

  .line > :nth-child(3) {
    grid-column: 1 / 3;
  }

  .total {
    justify-content: flex-start;
  }
}
</style>
