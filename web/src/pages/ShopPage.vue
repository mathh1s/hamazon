<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SiteFrame from '../components/SiteFrame.vue'
import CategoryTabs, { type Tab } from '../components/CategoryTabs.vue'
import ItemSlot from '../components/ItemSlot.vue'
import ItemDetail from '../components/ItemDetail.vue'
import CartChest from '../components/CartChest.vue'
import McButton from '../components/McButton.vue'
import McTextField from '../components/McTextField.vue'
import type { Item } from '../api'
import { catalog, itemsById, loadCatalog } from '../stores/catalog'
import { quantityInCart } from '../stores/cart'
import { hideTooltip, moveTooltip, showTooltip, type TooltipLine } from '../stores/tooltip'
import { priceText } from '../lib/price'
import { plural } from '../lib/format'
import { usePageTitle } from '../lib/title'

const SEARCH = '__search'
const route = useRoute()
const router = useRouter()

const tab = ref(typeof route.query.c === 'string' ? route.query.c : '')
const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const sortIndex = ref(0)
const selectedId = ref<number | null>(null)

onMounted(() => loadCatalog())

// Categories without any items for sale are left out of the tabs.
const tabs = computed<Tab[]>(() => [
  ...catalog.categories
    .filter((c) => catalog.items.some((i) => i.categoryId === c.id))
    .map((c) => ({ id: c.id, name: c.name, icon: c.icon })),
  { id: SEARCH, name: 'Search Items', icon: 'compass' },
])
const half = computed(() => Math.ceil(tabs.value.length / 2))

watch(
  tabs,
  (list) => {
    if (list.length <= 1) return
    if (!tab.value || !list.some((t) => t.id === tab.value)) tab.value = list[0]!.id
  },
  { immediate: true },
)

watch([tab, query], () => {
  router.replace({ query: { ...(tab.value ? { c: tab.value } : {}), ...(tab.value === SEARCH && query.value ? { q: query.value } : {}) } })
})

const currentTab = computed(() => tabs.value.find((t) => t.id === tab.value))
usePageTitle(() => currentTab.value?.name ?? 'Shop')

// There is no exchange rate between currencies, so instead of one "price"
// sort each currency gets its own; items without it go last.
const sorts = computed(() => [
  { label: 'Catalog', key: null as string | null },
  { label: 'Name', key: '__name' },
  ...catalog.currencies.map((c) => ({ label: c.name + 's', key: c.id })),
])
const sort = computed(() => sorts.value[sortIndex.value % sorts.value.length]!)

const visibleItems = computed(() => {
  let list: Item[]
  if (tab.value === SEARCH) {
    const q = query.value.trim().toLowerCase()
    list = q ? catalog.items.filter((i) => i.name.toLowerCase().includes(q)) : [...catalog.items]
  } else {
    list = catalog.items.filter((i) => i.categoryId === tab.value)
  }
  const key = sort.value.key
  if (key === '__name') return [...list].sort((a, b) => a.name.localeCompare(b.name))
  if (key) {
    return [...list].sort((a, b) => {
      const av = a.price[key]
      const bv = b.price[key]
      if (av === undefined && bv === undefined) return a.sort - b.sort
      if (av === undefined) return 1
      if (bv === undefined) return -1
      return av - bv || a.name.localeCompare(b.name)
    })
  }
  return list
})

const rows = computed(() => Math.max(3, Math.ceil(visibleItems.value.length / 9)))
const fillers = computed(() => rows.value * 9 - visibleItems.value.length)
const selected = computed(() => (selectedId.value !== null ? itemsById.value.get(selectedId.value) : undefined))

function select(item: Item) {
  selectedId.value = selectedId.value === item.id ? null : item.id
}

function selectFromCart(id: number) {
  const item = itemsById.value.get(id)
  if (!item) return
  tab.value = item.categoryId
  selectedId.value = id
}

function tooltipLines(item: Item): TooltipLine[] {
  const lines: TooltipLine[] = []
  if (!item.inStock) lines.push({ text: 'Out of stock', color: 'var(--red)' })
  if (item.description) lines.push({ text: item.description })
  if (item.count > 1) lines.push({ text: `Set of ${item.count}` })
  lines.push({ text: priceText(item.price, catalog.currencies), color: 'var(--gold)' })
  let limit = `Max ${item.maxPerOrder} per order`
  if (item.playerLimit) limit += `, ${item.playerLimit} per ${item.playerLimitDays} ${plural(item.playerLimitDays ?? 0, 'day', 'days')}`
  lines.push({ text: limit })
  const inCart = quantityInCart(item.id)
  if (inCart) lines.push({ text: `${inCart} in cart`, color: 'var(--green)' })
  return lines
}

function hover(e: PointerEvent, item: Item) {
  if (e.pointerType === 'mouse') showTooltip(e, item.name, tooltipLines(item))
}
</script>

<template>
  <SiteFrame wide>
    <div class="shop">
      <section class="inventory" aria-label="Items">
        <CategoryTabs :tabs="tabs.slice(0, half)" :selected="tab" position="top" @select="(id) => ((tab = id), (selectedId = null))" />
        <div class="mc-panel panel">
          <div class="head">
            <h2 class="mc-title">{{ currentTab?.name ?? 'Shop' }}</h2>
            <McButton small @click="sortIndex++">Sort: {{ sort.label }}</McButton>
          </div>
          <McTextField
            v-if="tab === SEARCH"
            v-model="query"
            class="search"
            type="search"
            placeholder="Search items"
            aria-label="Search items"
            autofocus
          />

          <p v-if="catalog.loading && !catalog.loaded" class="state">Loading items...</p>
          <div v-else-if="catalog.error" class="state">
            <p>{{ catalog.error }}</p>
            <McButton @click="loadCatalog(true)">Try again</McButton>
          </div>
          <template v-else>
            <div class="grid" role="list">
              <span v-for="item in visibleItems" :key="item.id" role="listitem">
                <ItemSlot
                  :texture="item.texture"
                  :count="item.count"
                  interactive
                  :selected="item.id === selectedId"
                  :dimmed="!item.inStock"
                  :label="`${item.name}, ${item.inStock ? priceText(item.price, catalog.currencies) : 'out of stock'}`"
                  @click="select(item)"
                  @pointerenter="hover($event, item)"
                  @pointermove="moveTooltip"
                  @pointerleave="hideTooltip"
                />
              </span>
              <ItemSlot v-for="i in fillers" :key="'f' + i" aria-hidden="true" />
            </div>
            <p v-if="visibleItems.length === 0" class="state">
              {{ tab === SEARCH && query ? `Nothing matches "${query}". Try another name.` : 'Nothing for sale here yet.' }}
            </p>
            <ItemDetail v-if="selected" :item="selected" />
            <p v-else-if="visibleItems.length" class="state">Select an item to see its price and buy it.</p>
          </template>
        </div>
        <CategoryTabs :tabs="tabs.slice(half)" :selected="tab" position="bottom" @select="(id) => ((tab = id), (selectedId = null))" />
      </section>

      <CartChest class="chest" @select="selectFromCart" />
    </div>
  </SiteFrame>
</template>

<style scoped>
.shop {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: flex-start;
  gap: calc(var(--u) * 8);
}

.inventory {
  width: calc(var(--u) * 176);
  max-width: 100%;
}

.panel {
  position: relative;
  z-index: 1;
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--u) * 2);
  margin-bottom: calc(var(--u) * 2);
}

.search {
  margin-bottom: calc(var(--u) * 2);
}

.grid {
  display: grid;
  grid-template-columns: repeat(9, calc(var(--u) * 18));
}

.grid > span {
  display: contents;
}

.state {
  margin: calc(var(--u) * 3) 0 0;
  color: var(--dark-gray);
}

.state p {
  margin: 0 0 calc(var(--u) * 3);
}

.chest {
  margin-top: calc(var(--u) * 24);
}

@media (max-width: 1179px) {
  .chest {
    margin-top: 0;
  }
}
</style>
