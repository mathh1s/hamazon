<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { Item } from '../api'
import ItemSlot from './ItemSlot.vue'
import PriceTag from './PriceTag.vue'
import McButton from './McButton.vue'
import QuantityStepper from './QuantityStepper.vue'
import { addToCart, quantityInCart } from '../stores/cart'
import { playPop } from '../stores/sound'
import { favorites, HOTBAR_SIZE, isFavorite, toggleFavorite } from '../stores/favorites'
import { scalePrice } from '../lib/price'
import { dimensionColor, needsText, plural } from '../lib/format'

const props = defineProps<{ item: Item }>()
const quantity = ref(1)
const added = ref('')

const inCart = computed(() => quantityInCart(props.item.id))
const room = computed(() => Math.max(0, props.item.maxPerOrder - inCart.value))

watch(
  () => props.item.id,
  () => {
    quantity.value = 1
    added.value = ''
  },
)

function add() {
  const n = Math.min(quantity.value, room.value)
  if (n < 1) return
  addToCart(props.item, n)
  playPop()
  added.value = `Added ${n} × ${props.item.name} to your cart`
  quantity.value = 1
}
</script>

<template>
  <section class="detail" aria-live="polite">
    <ItemSlot :texture="item.texture" :count="item.count" :size="26" :dimmed="!item.inStock" />
    <div class="info">
      <h3 class="mc-title">{{ item.name }}</h3>
      <p v-if="item.description" class="desc">{{ item.description }}</p>
      <p v-if="item.dimensions.length" class="dims" :style="{ color: dimensionColor[item.dimensions[item.dimensions.length - 1]!] }">
        {{ needsText(item.dimensions) }}
      </p>
      <p class="meta">
        {{ item.count === 1 ? 'Sold individually' : `Sold in sets of ${item.count}` }}
      </p>
      <PriceTag :price="item.price" class="item-price" />
      <p class="meta">
        Up to {{ item.maxPerOrder }} per order<template v-if="item.playerLimit"
          >, {{ item.playerLimit }} per player every {{ item.playerLimitDays }} {{ plural(item.playerLimitDays ?? 0, 'day', 'days') }}</template
        >
      </p>
    </div>
    <div class="buy">
      <p v-if="!item.inStock" class="soldout">Out of stock right now. Check back later.</p>
      <template v-else-if="room > 0">
        <QuantityStepper v-model="quantity" :max="room" label="Quantity" />
        <McButton @click="add">
          Add for
          <PriceTag :price="scalePrice(item.price, quantity)" />
        </McButton>
      </template>
      <p v-else class="meta">You have the most you can order of this in your cart.</p>
      <McButton
        small
        class="pin"
        :disabled="!isFavorite(item.id) && favorites.ids.length >= HOTBAR_SIZE"
        :aria-pressed="isFavorite(item.id)"
        @click="toggleFavorite(item.id)"
        >{{ isFavorite(item.id) ? 'Unpin' : favorites.ids.length >= HOTBAR_SIZE ? 'Hotbar full' : 'Pin to hotbar' }}</McButton
      >
      <p v-if="added || inCart" class="added" role="status">{{ added || `${inCart} in your cart` }}</p>
    </div>
  </section>
</template>

<style scoped>
.detail {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: calc(var(--u) * 4);
  padding-top: calc(var(--u) * 4);
  margin-top: calc(var(--u) * 4);
  border-top: var(--u) solid var(--panel-dark);
  box-shadow: inset 0 var(--u) 0 0 var(--panel-light);
}

.info {
  display: flex;
  flex-direction: column;
  gap: var(--u);
  min-width: 0;
}

.info p {
  margin: 0;
}

.dims {
  text-shadow: var(--u) var(--u) 0 #3f3f3f;
}

.desc {
  color: var(--dark-gray);
}

.meta {
  color: var(--dark-gray);
}

.item-price {
  color: var(--title);
}

.buy {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: calc(var(--u) * 3);
}

.buy p {
  margin: 0;
}

.pin {
  margin-left: auto;
}

.soldout {
  color: #aa0000;
}

.added {
  flex-basis: 100%;
  color: var(--title);
}
</style>
