<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AdminFrame from '../../components/AdminFrame.vue'
import McButton from '../../components/McButton.vue'
import McTextField from '../../components/McTextField.vue'
import ItemSlot from '../../components/ItemSlot.vue'
import PriceTag from '../../components/PriceTag.vue'
import { api, errorMessage, type Dimension, type Item } from '../../api'
import { dimensionLabel } from '../../lib/format'
import { admin, loadOverview } from '../../stores/admin'
import { loadCatalog } from '../../stores/catalog'
import { usePageTitle } from '../../lib/title'

usePageTitle(() => 'Items')
const items = ref<Item[]>([])
const missingDefaults = ref(0)
const dimensions = ref<{ id: Dimension; enabled: boolean; items: number }[]>([])
const justAdded = ref<number[]>([])
const adding = ref(false)
const error = ref('')
const loading = ref(true)
const search = ref('')

onMounted(async () => {
  loadCatalog()
  try {
    if (!admin.loaded) await loadOverview()
    await reload()
    missingDefaults.value = (await api<{ missing: unknown[] }>('GET', '/api/admin/defaults')).missing.length
    dimensions.value = (await api<{ dimensions: typeof dimensions.value }>('GET', '/api/admin/dimensions')).dimensions
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
})

async function toggleDimension(d: { id: Dimension; enabled: boolean }) {
  if (d.enabled && !confirm(`Hide every item that needs the ${dimensionLabel[d.id]} from the shop? Nothing is deleted.`)) return
  try {
    dimensions.value = (
      await api<{ dimensions: typeof dimensions.value }>('PUT', `/api/admin/dimensions/${d.id}`, { enabled: !d.enabled })
    ).dimensions
    loadCatalog(true)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function reload() {
  items.value = (await api<{ items: Item[] }>('GET', '/api/admin/items')).items
}

async function addDefaults() {
  adding.value = true
  error.value = ''
  try {
    const res = await api<{ added: number[] }>('POST', '/api/admin/defaults')
    justAdded.value = res.added
    missingDefaults.value = 0
    await loadOverview()
    await reload()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    adding.value = false
  }
}

async function showAdded() {
  if (!confirm(`Show all ${justAdded.value.length} new items in the shop with their default prices?`)) return
  try {
    await api('POST', '/api/admin/items/visibility', { ids: justAdded.value, enabled: true })
    justAdded.value = []
    await reload()
    loadCatalog(true)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const toggling = ref<number | null>(null)

async function toggleStock(item: Item) {
  toggling.value = item.id
  try {
    const updated = await api<Item>('PUT', `/api/admin/items/${item.id}/stock`, { inStock: !item.inStock })
    items.value = items.value.map((i) => (i.id === updated.id ? updated : i))
    loadCatalog(true)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    toggling.value = null
  }
}

// With well over a thousand items, categories start folded so the page
// stays quick; a filter opens every category that has a match.
const open = ref<Set<string>>(new Set())

function toggleGroup(id: string) {
  const next = new Set(open.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  open.value = next
}

const groups = computed(() => {
  const q = search.value.trim().toLowerCase()
  return admin.categories
    .map((c) => ({
      category: c,
      items: items.value.filter((i) => i.categoryId === c.id && (!q || i.name.toLowerCase().includes(q) || i.texture.includes(q))),
    }))
    .filter((g) => g.items.length > 0 || !q)
})
</script>

<template>
  <AdminFrame>
    <section class="mc-panel items" aria-labelledby="items-title">
      <div class="head">
        <h2 id="items-title" class="mc-title">Items</h2>
        <McButton to="/admin/items/new">Add item</McButton>
      </div>
      <div v-if="missingDefaults > 0" class="notice">
        <p>
          {{ missingDefaults }} default {{ missingDefaults === 1 ? 'item is' : 'items are' }} not in your shop yet. They are added
          hidden so you can check the prices first.
        </p>
        <McButton :disabled="adding" @click="addDefaults">{{ adding ? 'Adding...' : 'Add missing defaults' }}</McButton>
      </div>
      <div v-if="justAdded.length" class="notice" role="status">
        <p>Added {{ justAdded.length }} hidden items. Edit their prices, or show them all at once.</p>
        <McButton @click="showAdded">Show all {{ justAdded.length }} in the shop</McButton>
      </div>
      <div v-if="dimensions.length" class="dims">
        <span class="dims-label">Dimensions open on the server</span>
        <McButton v-for="d in dimensions" :key="d.id" small :aria-pressed="d.enabled" @click="toggleDimension(d)"
          >{{ dimensionLabel[d.id] }}: {{ d.enabled ? 'ON' : 'OFF' }}</McButton
        >
        <span class="dims-hint">Items that need a dimension that is off ({{
          dimensions.filter((d) => !d.enabled).reduce((n, d) => n + d.items, 0)
        }}) are hidden from the shop.</span>
      </div>
      <McTextField v-model="search" type="search" placeholder="Filter items" aria-label="Filter items" />
      <p v-if="loading" class="hint">Loading items...</p>
      <p v-else-if="error" class="error" role="alert">{{ error }}</p>

      <section v-for="g in groups" :key="g.category.id" class="group" :aria-label="g.category.name">
        <button
          type="button"
          class="group-head"
          :aria-expanded="!!search || open.has(g.category.id)"
          @click="toggleGroup(g.category.id)"
        >
          <span class="arrow" aria-hidden="true">{{ search || open.has(g.category.id) ? 'v' : '>' }}</span>
          <span class="mc-title">{{ g.category.name }}</span>
          <span class="count">
            {{ g.items.length }} items<template v-if="g.items.some((i) => !i.enabled)">,
              {{ g.items.filter((i) => !i.enabled).length }} hidden</template
            >
          </span>
        </button>
        <template v-if="search || open.has(g.category.id)">
          <p v-if="g.items.length === 0" class="hint">No items in this category.</p>
          <ul class="list">
            <li v-for="item in g.items" :key="item.id" class="entry">
              <RouterLink :to="`/admin/items/${item.id}`" class="row" :class="{ disabled: !item.enabled }">
                <ItemSlot :texture="item.texture" :count="item.count" :dimmed="!item.inStock" />
                <span class="name"
                  >{{ item.name }}<span v-if="!item.enabled" class="off"> (hidden)</span
                  ><span v-else-if="!item.inStock" class="soldout"> (out of stock)</span></span
                >
                <PriceTag :price="item.price" />
                <span class="limit">
                  Max {{ item.maxPerOrder }} per order<template v-if="item.playerLimit">, {{ item.playerLimit }} per {{ item.playerLimitDays }} days</template>
                </span>
              </RouterLink>
              <McButton
                small
                class="stock"
                :disabled="toggling === item.id"
                :aria-label="`${item.inStock ? 'Mark out of stock' : 'Mark in stock'}: ${item.name}`"
                :aria-pressed="item.inStock"
                @click="toggleStock(item)"
                >Stock: {{ item.inStock ? 'ON' : 'OFF' }}</McButton
              >
            </li>
          </ul>
        </template>
      </section>
    </section>
  </AdminFrame>
</template>

<style scoped>
.items {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 4);
}

.head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.dims {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: calc(var(--u) * 2) calc(var(--u) * 3);
}

.dims-label {
  color: var(--title);
}

.dims-hint {
  flex-basis: 100%;
  color: var(--dark-gray);
}

.notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--u) * 3);
  padding: calc(var(--u) * 3);
  background: var(--slot-dark);
  color: var(--text);
  text-shadow: var(--u) var(--u) 0 var(--text-shadow);
}

.notice p {
  margin: 0;
  flex: 1 1 calc(var(--u) * 140);
}

.hint {
  margin: 0;
  color: var(--dark-gray);
}

.error {
  margin: 0;
  color: #aa0000;
}

.group-head {
  display: flex;
  align-items: baseline;
  gap: calc(var(--u) * 3);
  width: 100%;
  padding: var(--u) 0;
  border: 0;
  background: none;
  cursor: pointer;
  text-align: left;
}

.group-head:hover .mc-title {
  color: #000;
}

.arrow,
.count {
  color: var(--dark-gray);
}

.group {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(calc(var(--u) * 160), 1fr));
  gap: var(--u);
}

.entry {
  display: flex;
  gap: var(--u);
}

.entry .row {
  flex: 1;
  min-width: 0;
}

.stock {
  align-self: stretch;
  min-width: calc(var(--u) * 58);
}

.soldout {
  color: var(--red);
}

.row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  grid-template-rows: auto auto;
  column-gap: calc(var(--u) * 3);
  align-items: center;
  padding: var(--u) calc(var(--u) * 2);
  background: var(--slot-dark);
  color: var(--text);
  text-decoration: none;
  text-shadow: var(--u) var(--u) 0 var(--text-shadow);
}

.row > :first-child {
  grid-row: 1 / 3;
}

.row:hover,
.row:focus-visible {
  background: #4a4a4a;
  outline: var(--u) solid var(--text);
}

.row.disabled {
  opacity: 0.6;
}

.off,
.limit {
  color: var(--gray);
}

.limit {
  grid-column: 2 / 4;
}
</style>
