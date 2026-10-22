<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AdminFrame from '../../components/AdminFrame.vue'
import McButton from '../../components/McButton.vue'
import ItemSlot from '../../components/ItemSlot.vue'
import PriceTag from '../../components/PriceTag.vue'
import { api, errorMessage, type PriceTerms, type PriceUpdate } from '../../api'
import { loadOverview } from '../../stores/admin'
import { loadCatalog } from '../../stores/catalog'
import { usePageTitle } from '../../lib/title'

usePageTitle(() => 'Price updates')
const updates = ref<PriceUpdate[]>([])
const selected = ref<Set<number>>(new Set())
const showLocked = ref(false)
const loading = ref(true)
const applying = ref(false)
const error = ref('')
const done = ref('')

const lockedCount = computed(() => updates.value.filter((u) => u.locked).length)
const visible = computed(() => updates.value.filter((u) => showLocked.value || !u.locked))

async function load() {
  loading.value = true
  error.value = ''
  try {
    updates.value = (await api<{ updates: PriceUpdate[] }>('GET', '/api/admin/price-updates')).updates
    // Items priced by hand stay as they are unless picked on purpose.
    selected.value = new Set(updates.value.filter((u) => !u.locked).map((u) => u.itemId))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function toggle(id: number) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

function selectAll(on: boolean) {
  selected.value = on ? new Set(visible.value.map((u) => u.itemId)) : new Set()
}

async function apply() {
  applying.value = true
  error.value = ''
  try {
    const res = await api<{ applied: number }>('POST', '/api/admin/price-updates', { ids: [...selected.value] })
    done.value = `Updated ${res.applied} ${res.applied === 1 ? 'item' : 'items'}.`
    await Promise.all([load(), loadOverview(), loadCatalog(true)])
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    applying.value = false
  }
}

function limitText(t: PriceTerms) {
  let s = `${t.count === 1 ? 'single' : `set of ${t.count}`}, max ${t.maxPerOrder}`
  if (t.playerLimit) s += `, ${t.playerLimit} per ${t.playerLimitDays}d`
  return s
}

onMounted(() => {
  loadCatalog()
  load()
})
</script>

<template>
  <AdminFrame>
    <section class="mc-panel prices" aria-labelledby="prices-title">
      <h2 id="prices-title" class="mc-title">Price updates</h2>
      <p class="hint">
        These items cost something different from the current Hamazon defaults. Pick the ones to update; orders already placed
        keep their price.
      </p>

      <p v-if="loading" class="hint">Comparing prices...</p>
      <p v-else-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="done" class="ok" role="status">{{ done }}</p>

      <template v-if="!loading && !error">
        <p v-if="visible.length === 0" class="hint">
          Everything matches the defaults{{ lockedCount ? ', apart from prices you set yourself' : '' }}.
        </p>
        <div v-else class="toolbar">
          <McButton small @click="selectAll(true)">Select all</McButton>
          <McButton small @click="selectAll(false)">Select none</McButton>
          <McButton :disabled="applying || selected.size === 0" @click="apply">
            {{ applying ? 'Updating...' : `Update ${selected.size} ${selected.size === 1 ? 'price' : 'prices'}` }}
          </McButton>
        </div>
        <label v-if="lockedCount" class="check">
          <input v-model="showLocked" type="checkbox" />
          <span>Also show {{ lockedCount }} {{ lockedCount === 1 ? 'price' : 'prices' }} you set yourself</span>
        </label>

        <ul class="list">
          <li v-for="u in visible" :key="u.itemId">
            <label class="row" :class="{ off: !selected.has(u.itemId) }">
              <input type="checkbox" :checked="selected.has(u.itemId)" @change="toggle(u.itemId)" />
              <ItemSlot :texture="u.texture" :count="u.default.count" />
              <span class="name">
                <span>{{ u.name }}<span v-if="u.locked" class="locked"> (set by you)</span></span>
                <span v-if="limitText(u.current) !== limitText(u.default)" class="terms"
                  >{{ limitText(u.current) }} → {{ limitText(u.default) }}</span
                >
              </span>
              <span class="change">
                <PriceTag :price="u.current.price" class="old" />
                <span aria-hidden="true">→</span>
                <span class="visually-hidden">becomes</span>
                <PriceTag :price="u.default.price" />
              </span>
            </label>
          </li>
        </ul>
      </template>
    </section>
  </AdminFrame>
</template>

<style scoped>
.prices {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 4);
}

.hint {
  margin: 0;
  color: var(--dark-gray);
}

.error {
  margin: 0;
  color: #aa0000;
}

.ok {
  margin: 0;
  color: #00aa00;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: calc(var(--u) * 2);
}

.check {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 3);
  color: var(--title);
  cursor: pointer;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--u);
}

.row {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  align-items: center;
  gap: calc(var(--u) * 3);
  padding: var(--u) calc(var(--u) * 3);
  background: var(--slot-dark);
  color: var(--text);
  text-shadow: var(--u) var(--u) 0 var(--text-shadow);
  cursor: pointer;
}

.row.off {
  opacity: 0.6;
}

input[type='checkbox'] {
  width: calc(var(--u) * 10);
  height: calc(var(--u) * 10);
  accent-color: #555;
}

.name {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.terms,
.locked {
  color: var(--gray);
}

.change {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 3);
}

.old {
  opacity: 0.7;
}

@media (max-width: 899px) {
  .row {
    grid-template-columns: auto auto minmax(0, 1fr);
  }

  .change {
    grid-column: 2 / -1;
  }
}
</style>
