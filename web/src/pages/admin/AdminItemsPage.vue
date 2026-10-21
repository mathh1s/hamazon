<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AdminFrame from '../../components/AdminFrame.vue'
import McButton from '../../components/McButton.vue'
import McTextField from '../../components/McTextField.vue'
import ItemSlot from '../../components/ItemSlot.vue'
import PriceTag from '../../components/PriceTag.vue'
import { api, errorMessage, type Item } from '../../api'
import { admin, loadOverview } from '../../stores/admin'
import { loadCatalog } from '../../stores/catalog'
import { usePageTitle } from '../../lib/title'

usePageTitle(() => 'Items')
const items = ref<Item[]>([])
const error = ref('')
const loading = ref(true)
const search = ref('')

onMounted(async () => {
  loadCatalog()
  try {
    if (!admin.loaded) await loadOverview()
    items.value = (await api<{ items: Item[] }>('GET', '/api/admin/items')).items
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
})

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
      <McTextField v-model="search" type="search" placeholder="Filter items" aria-label="Filter items" />
      <p v-if="loading" class="hint">Loading items...</p>
      <p v-else-if="error" class="error" role="alert">{{ error }}</p>

      <section v-for="g in groups" :key="g.category.id" class="group" :aria-label="g.category.name">
        <h3 class="mc-title">{{ g.category.name }}</h3>
        <p v-if="g.items.length === 0" class="hint">No items in this category.</p>
        <ul class="list">
          <li v-for="item in g.items" :key="item.id">
            <RouterLink :to="`/admin/items/${item.id}`" class="row" :class="{ disabled: !item.enabled }">
              <ItemSlot :texture="item.texture" :count="item.count" />
              <span class="name">{{ item.name }}<span v-if="!item.enabled" class="off"> (hidden)</span></span>
              <PriceTag :price="item.price" />
              <span class="limit">
                Max {{ item.maxPerOrder }} per order<template v-if="item.playerLimit">, {{ item.playerLimit }} per {{ item.playerLimitDays }} days</template>
              </span>
            </RouterLink>
          </li>
        </ul>
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

.hint {
  margin: 0;
  color: var(--dark-gray);
}

.error {
  margin: 0;
  color: #aa0000;
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
