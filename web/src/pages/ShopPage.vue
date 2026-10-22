<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SiteFrame from '../components/SiteFrame.vue'
import CategoryTabs, { type Tab } from '../components/CategoryTabs.vue'
import ItemSlot from '../components/ItemSlot.vue'
import ItemIcon from '../components/ItemIcon.vue'
import ItemDetail from '../components/ItemDetail.vue'
import CartChest from '../components/CartChest.vue'
import FavoritesHotbar from '../components/FavoritesHotbar.vue'
import McButton from '../components/McButton.vue'
import McTextField from '../components/McTextField.vue'
import type { Item } from '../api'
import { catalog, itemsById, loadCatalog } from '../stores/catalog'
import { addToCart, quantityInCart } from '../stores/cart'
import { playPop } from '../stores/sound'
import { hideTooltip, moveTooltip, showTooltip, type TooltipLine } from '../stores/tooltip'
import { priceText } from '../lib/price'
import { dimensionColor, dimensionLabel, needsText, plural } from '../lib/format'
import { usePageTitle } from '../lib/title'

const SEARCH = '__search'
const route = useRoute()
const router = useRouter()

const tab = ref(typeof route.query.c === 'string' ? route.query.c : '')
const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const sortIndex = ref(0)
const dimIndex = ref(0)
const payIndex = ref(0)
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

// Only dimensions the server has open are offered as filters.
const DIM_ICONS = { overworld: 'grass_block', nether: 'netherrack', end: 'end_stone' } as const
const dimFilters = computed(() => [
  { label: 'All', id: null as string | null, icon: 'compass' },
  { label: dimensionLabel.overworld, id: 'overworld', icon: DIM_ICONS.overworld },
  ...(['nether', 'end'] as const)
    .filter((d) => !catalog.disabledDimensions.includes(d))
    .map((d) => ({ label: dimensionLabel[d], id: d as string, icon: DIM_ICONS[d] })),
])
const dimFilter = computed(() => dimFilters.value[dimIndex.value % dimFilters.value.length]!)

// Players often have just one kind of currency on them, so they can show
// only what they can pay for with it.
const payFilters = computed(() => [
  { label: 'Any', id: null as string | null, icon: 'bundle' },
  ...catalog.currencies.map((c) => ({ label: c.name + 's', id: c.id as string | null, icon: c.texture })),
])
const payFilter = computed(() => payFilters.value[payIndex.value % payFilters.value.length]!)

function matchesPay(item: Item) {
  const c = payFilter.value.id
  return !c || Object.keys(item.price).every((k) => k === c)
}

function matchesDimension(item: Item) {
  const d = dimFilter.value.id
  if (!d) return true
  if (d === 'overworld') return item.dimensions.length === 0
  return item.dimensions.includes(d as 'nether' | 'end')
}

const visibleItems = computed(() => {
  let list: Item[]
  if (tab.value === SEARCH) {
    const q = query.value.trim().toLowerCase()
    list = q ? catalog.items.filter((i) => i.name.toLowerCase().includes(q)) : [...catalog.items]
  } else {
    list = catalog.items.filter((i) => i.categoryId === tab.value)
  }
  list = list.filter((i) => matchesDimension(i) && matchesPay(i))
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

// In a category, the variants of a group (every shulker box color, every
// wood type...) share one slot. Search lists everything on its own.
type Entry = { kind: 'item'; item: Item } | { kind: 'group'; name: string; items: Item[] }

const entries = computed<Entry[]>(() => {
  if (tab.value === SEARCH) return visibleItems.value.map((item) => ({ kind: 'item' as const, item }))
  const groups = new Map<string, Item[]>()
  for (const item of visibleItems.value) {
    if (!item.group) continue
    const list = groups.get(item.group) ?? []
    list.push(item)
    groups.set(item.group, list)
  }
  const out: Entry[] = []
  const emitted = new Set<string>()
  for (const item of visibleItems.value) {
    const members = item.group ? groups.get(item.group)! : undefined
    if (!members || members.length < 2) {
      out.push({ kind: 'item', item })
    } else if (!emitted.has(item.group)) {
      emitted.add(item.group)
      out.push({ kind: 'group', name: item.group, items: members })
    }
  }
  return out
})

const openGroup = ref<string | null>(null)
const openGroupItems = computed(() => {
  const e = entries.value.find((x) => x.kind === 'group' && x.name === openGroup.value)
  return e && e.kind === 'group' ? e.items : []
})
watch([tab, dimIndex, payIndex], () => (openGroup.value = null))

function toggleGroup(name: string) {
  openGroup.value = openGroup.value === name ? null : name
}

function groupHover(e: PointerEvent, name: string, items: Item[]) {
  if (e.pointerType !== 'mouse') return
  showTooltip(e, name, [{ text: `${items.length} variants, click to pick one` }])
}

const rows = computed(() => Math.max(5, Math.ceil(entries.value.length / 9)))
const fillers = computed(() => rows.value * 9 - entries.value.length)
const selected = computed(() => (selectedId.value !== null ? itemsById.value.get(selectedId.value) : undefined))

// Shift-click moves a stack in the game; here it puts one set straight
// into the cart.
function quickAdd(item: Item) {
  if (!item.inStock || quantityInCart(item.id) >= item.maxPerOrder) {
    selectedId.value = item.id
    return
  }
  addToCart(item, 1)
  playPop()
  selectedId.value = item.id
}

function onSlotClick(e: MouseEvent, item: Item) {
  if (e.shiftKey) quickAdd(item)
  else select(item)
}

function select(item: Item) {
  selectedId.value = selectedId.value === item.id ? null : item.id
}

const gridEl = ref<HTMLElement | null>(null)

async function selectFromCart(id: number) {
  const item = itemsById.value.get(id)
  if (!item) return
  tab.value = item.categoryId
  selectedId.value = id
  openGroup.value = item.group || null
  await nextTick()
  gridEl.value?.querySelector('.slot.selected')?.scrollIntoView({ block: 'nearest' })
}

function tooltipLines(item: Item): TooltipLine[] {
  const lines: TooltipLine[] = []
  if (!item.inStock) lines.push({ text: 'Out of stock', color: 'var(--red)' })
  if (item.description) lines.push({ text: item.description })
  if (item.count > 1) lines.push({ text: `Set of ${item.count}` })
  if (item.dimensions.length) lines.push({ text: needsText(item.dimensions), color: dimensionColor[item.dimensions[item.dimensions.length - 1]!] })
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
          <h2 class="mc-title head">{{ currentTab?.name ?? 'Shop' }}</h2>
          <div class="tools">
            <McButton small :aria-label="`Dimension: ${dimFilter.label}`" @click="dimIndex++">
              <ItemIcon :texture="dimFilter.icon" :size="8" />{{ dimFilter.label }}
            </McButton>
            <McButton small :aria-label="`Pay with: ${payFilter.label}`" @click="payIndex++">
              <ItemIcon :texture="payFilter.icon" :size="8" />{{ payFilter.id ? 'Only' : 'Any price' }}
            </McButton>
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
            <div ref="gridEl" class="grid mc-scroll" role="list">
              <span
                v-for="e in entries"
                :key="e.kind === 'item' ? e.item.id : 'g:' + e.name"
                role="listitem"
              >
                <ItemSlot
                  v-if="e.kind === 'item'"
                  :texture="e.item.texture"
                  :count="e.item.count"
                  interactive
                  :selected="e.item.id === selectedId"
                  :dimmed="!e.item.inStock"
                  :label="`${e.item.name}, ${e.item.inStock ? priceText(e.item.price, catalog.currencies) : 'out of stock'}`"
                  @click="onSlotClick($event, e.item)"
                  @pointerenter="hover($event, e.item)"
                  @pointermove="moveTooltip"
                  @pointerleave="hideTooltip"
                />
                <ItemSlot
                  v-else
                  class="group-slot"
                  :texture="e.items[0]!.texture"
                  interactive
                  :selected="openGroup === e.name"
                  :label="`${e.name}, ${e.items.length} variants`"
                  :aria-expanded="openGroup === e.name"
                  @click="toggleGroup(e.name)"
                  @pointerenter="groupHover($event, e.name, e.items)"
                  @pointermove="moveTooltip"
                  @pointerleave="hideTooltip"
                />
              </span>
              <ItemSlot v-for="i in fillers" :key="'f' + i" aria-hidden="true" />
            </div>
            <section v-if="openGroup && openGroupItems.length" class="drawer" :aria-label="openGroup">
              <div class="drawer-head">
                <span>{{ openGroup }}</span>
                <button type="button" class="drawer-close" aria-label="Close" @click="openGroup = null">x</button>
              </div>
              <div class="drawer-grid">
                <ItemSlot
                  v-for="item in openGroupItems"
                  :key="item.id"
                  :texture="item.texture"
                  :count="item.count"
                  interactive
                  :selected="item.id === selectedId"
                  :dimmed="!item.inStock"
                  :label="`${item.name}, ${item.inStock ? priceText(item.price, catalog.currencies) : 'out of stock'}`"
                  @click="onSlotClick($event, item)"
                  @pointerenter="hover($event, item)"
                  @pointermove="moveTooltip"
                  @pointerleave="hideTooltip"
                />
              </div>
            </section>
            <p v-if="visibleItems.length === 0" class="state">
              {{
                tab === SEARCH && query
                  ? `Nothing matches "${query}". Try another name.`
                  : dimFilter.id || payFilter.id
                    ? 'Nothing here matches these filters.'
                    : 'Nothing for sale here yet.'
              }}
            </p>
            <ItemDetail v-if="selected" :item="selected" />
            <p v-else-if="visibleItems.length" class="state">
              Select an item to see its price and buy it. Shift-click adds a set straight to the cart.
            </p>
          </template>
        </div>
        <CategoryTabs :tabs="tabs.slice(half)" :selected="tab" position="bottom" @select="(id) => ((tab = id), (selectedId = null))" />
        <FavoritesHotbar :selected-id="selectedId" @select="(item) => selectFromCart(item.id)" />
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
  width: calc(var(--u) * 195);
  max-width: 100%;
}

.panel {
  position: relative;
  z-index: 1;
}

.head {
  margin-bottom: calc(var(--u) * 2);
}

.tools {
  display: flex;
  flex-wrap: wrap;
  gap: var(--u);
  margin-bottom: calc(var(--u) * 2);
}

.search {
  margin-bottom: calc(var(--u) * 2);
}

/* Nine rows at a time, the rest scrolls, like the creative inventory. */
.grid {
  display: grid;
  grid-template-columns: repeat(9, calc(var(--u) * 18));
  grid-auto-rows: calc(var(--u) * 18);
  max-height: calc(var(--u) * 18 * 9);
  width: fit-content;
  padding-right: calc(var(--u) * 2);
}

.grid > span {
  display: contents;
}

/* A small corner fold marks slots that open into several variants. */
.grid :deep(.group-slot)::before {
  content: '';
  position: absolute;
  top: var(--u);
  left: var(--u);
  z-index: 1;
  border-style: solid;
  border-width: calc(var(--u) * 4) calc(var(--u) * 4) 0 0;
  border-color: var(--yellow) transparent transparent transparent;
  pointer-events: none;
}

/* Styled after the bundle tooltip: a dark box with the contents. */
.drawer {
  margin-top: calc(var(--u) * 3);
  padding: calc(var(--u) * 2) calc(var(--u) * 3) calc(var(--u) * 3);
  background: var(--tooltip-bg);
  border: var(--u) solid rgba(80, 0, 255, 0.6);
  color: var(--text);
  text-shadow: var(--u) var(--u) 0 var(--text-shadow);
}

.drawer-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: calc(var(--u) * 2);
}

.drawer-close {
  padding: 0 calc(var(--u) * 2);
  border: 0;
  background: none;
  color: var(--gray);
  cursor: pointer;
}

.drawer-close:hover {
  color: var(--text);
}

.drawer-grid {
  display: grid;
  grid-template-columns: repeat(9, calc(var(--u) * 18));
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
