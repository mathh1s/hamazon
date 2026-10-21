<script setup lang="ts">
import { computed } from 'vue'
import type { Price } from '../api'
import { catalog } from '../stores/catalog'
import { currencyLabel, priceParts } from '../lib/price'
import ItemIcon from './ItemIcon.vue'

const props = withDefaults(defineProps<{ price: Price; stacked?: boolean; iconSize?: number }>(), { iconSize: 8 })
const parts = computed(() => priceParts(props.price, catalog.currencies))
</script>

<template>
  <span class="price" :class="{ stacked }">
    <template v-for="(p, i) in parts" :key="p.currency.id">
      <span v-if="i > 0 && !stacked" class="plus" aria-hidden="true">+</span>
      <span class="part">
        <ItemIcon :texture="p.currency.texture" :size="iconSize" />
        <span>{{ p.amount }}</span>
        <span class="visually-hidden">{{ currencyLabel(p.currency, p.amount) }}</span>
      </span>
    </template>
  </span>
</template>

<style scoped>
.price {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: calc(var(--u) * 2);
}

.price.stacked {
  flex-direction: column;
  align-items: flex-start;
  gap: var(--u);
}

.part {
  display: inline-flex;
  align-items: center;
  gap: var(--u);
}
</style>
