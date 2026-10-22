<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AdminFrame from '../../components/AdminFrame.vue'
import McButton from '../../components/McButton.vue'
import McTextField from '../../components/McTextField.vue'
import ItemSlot from '../../components/ItemSlot.vue'
import TexturePicker from '../../components/TexturePicker.vue'
import { api, ApiError, errorMessage, type Item, type ItemInput } from '../../api'
import { admin, loadOverview } from '../../stores/admin'
import { loadCatalog } from '../../stores/catalog'
import { usePageTitle } from '../../lib/title'

const route = useRoute()
const router = useRouter()
const isNew = computed(() => route.params.id === undefined)
const id = computed(() => Number(route.params.id))

usePageTitle(() => (isNew.value ? 'Add item' : 'Edit item'))

// Text inputs keep strings so an empty field is not silently turned into 0.
const form = reactive({
  name: '',
  texture: '',
  categoryId: '',
  count: '1',
  description: '',
  maxPerOrder: '1',
  limitOn: false,
  playerLimit: '1',
  playerLimitDays: '7',
  enabled: true,
  inStock: true,
  sort: '0',
  price: {} as Record<string, string>,
})
const errors = ref<Record<string, string>>({})
const error = ref('')
const loading = ref(true)
const saving = ref(false)
const saved = ref('')

onMounted(async () => {
  try {
    if (!admin.loaded) await loadOverview()
    form.categoryId = admin.categories[0]?.id ?? ''
    if (!isNew.value) {
      const items = (await api<{ items: Item[] }>('GET', '/api/admin/items')).items
      const item = items.find((i) => i.id === id.value)
      if (!item) {
        error.value = 'This item does not exist anymore.'
        return
      }
      Object.assign(form, {
        name: item.name,
        texture: item.texture,
        categoryId: item.categoryId,
        count: String(item.count),
        description: item.description,
        maxPerOrder: String(item.maxPerOrder),
        limitOn: item.playerLimit !== null,
        playerLimit: String(item.playerLimit ?? 1),
        playerLimitDays: String(item.playerLimitDays ?? 7),
        enabled: item.enabled,
        inStock: item.inStock,
        sort: String(item.sort),
        price: Object.fromEntries(Object.entries(item.price).map(([k, v]) => [k, String(v)])),
      })
    }
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
})

const usedCurrencies = computed(() => admin.currencies.filter((c) => Number(form.price[c.id]) > 0).length)

function onPicked(p: { name: string }) {
  if (!form.name) form.name = p.name
}

function toInput(): ItemInput {
  const price: Record<string, number> = {}
  for (const c of admin.currencies) {
    const n = Number(form.price[c.id])
    if (n > 0) price[c.id] = Math.floor(n)
  }
  return {
    name: form.name,
    texture: form.texture,
    categoryId: form.categoryId,
    count: Number(form.count),
    description: form.description,
    maxPerOrder: Number(form.maxPerOrder),
    playerLimit: form.limitOn ? Number(form.playerLimit) : null,
    playerLimitDays: form.limitOn ? Number(form.playerLimitDays) : null,
    enabled: form.enabled,
    inStock: form.inStock,
    sort: Number(form.sort) || 0,
    price,
  }
}

async function save() {
  saving.value = true
  error.value = ''
  saved.value = ''
  errors.value = {}
  try {
    const input = toInput()
    if (isNew.value) {
      const item = await api<Item>('POST', '/api/admin/items', input)
      await loadCatalog(true)
      router.replace(`/admin/items/${item.id}`)
      saved.value = 'Item added to the shop'
    } else {
      await api<Item>('PUT', `/api/admin/items/${id.value}`, input)
      await loadCatalog(true)
      saved.value = 'Changes saved'
    }
  } catch (e) {
    if (e instanceof ApiError) errors.value = e.fields
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!confirm(`Remove ${form.name} from the shop? Existing orders keep their copy of it.`)) return
  try {
    await api('DELETE', `/api/admin/items/${id.value}`)
    await loadCatalog(true)
    router.replace('/admin/items')
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <AdminFrame>
    <form class="mc-panel editor" novalidate @submit.prevent="save">
      <div class="head">
        <h2 class="mc-title">{{ isNew ? 'Add item' : 'Edit item' }}</h2>
        <McButton small to="/admin/items">Back to items</McButton>
      </div>
      <p v-if="loading" class="hint">Loading...</p>

      <template v-else>
        <div class="preview">
          <ItemSlot :texture="form.texture || undefined" :count="Number(form.count)" :size="26" :dimmed="!form.inStock" />
          <span>{{ form.name || 'Unnamed item' }}</span>
        </div>

        <div class="field">
          <span class="label">Texture</span>
          <TexturePicker v-model="form.texture" @picked="onPicked" />
          <McTextField id="texture" v-model="form.texture" :invalid="!!errors.texture" aria-label="Texture id" placeholder="item id, e.g. diamond_pickaxe" />
          <p v-if="errors.texture" class="error">{{ errors.texture }}</p>
        </div>

        <div class="cols">
          <div class="field">
            <label for="name" class="label">Name</label>
            <McTextField id="name" v-model="form.name" maxlength="64" :invalid="!!errors.name" />
            <p v-if="errors.name" class="error">{{ errors.name }}</p>
          </div>
          <div class="field">
            <label for="category" class="label">Category</label>
            <select id="category" v-model="form.categoryId" class="select shadow">
              <option v-for="c in admin.categories" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
            <p v-if="errors.categoryId" class="error">{{ errors.categoryId }}</p>
          </div>
        </div>

        <div class="field">
          <label for="desc" class="label">Description (optional, shown in the tooltip)</label>
          <McTextField id="desc" v-model="form.description" maxlength="200" :invalid="!!errors.description" />
          <p v-if="errors.description" class="error">{{ errors.description }}</p>
        </div>

        <fieldset class="field">
          <legend class="label">Price per set (one or two currencies)</legend>
          <div class="cols three">
            <label v-for="c in admin.currencies" :key="c.id" class="currency">
              <ItemSlot :texture="c.texture" />
              <McTextField
                :model-value="form.price[c.id] ?? ''"
                @update:model-value="(v: string) => (form.price[c.id] = v)"
                inputmode="numeric"
                :aria-label="`${c.name}s`"
                :placeholder="c.name + 's'"
                :invalid="!!errors.price"
              />
            </label>
          </div>
          <p v-if="errors.price" class="error">{{ errors.price }}</p>
          <p v-else-if="usedCurrencies > 2" class="error">Use at most two currencies.</p>
          <p v-else class="hint">Leave a currency empty if the item does not cost it.</p>
        </fieldset>

        <div class="cols three">
          <div class="field">
            <label for="count" class="label">Items per set</label>
            <McTextField id="count" v-model="form.count" inputmode="numeric" :invalid="!!errors.count" />
            <p v-if="errors.count" class="error">{{ errors.count }}</p>
          </div>
          <div class="field">
            <label for="max" class="label">Max sets per order</label>
            <McTextField id="max" v-model="form.maxPerOrder" inputmode="numeric" :invalid="!!errors.maxPerOrder" />
            <p v-if="errors.maxPerOrder" class="error">{{ errors.maxPerOrder }}</p>
          </div>
          <div class="field">
            <label for="sort" class="label">Position in category</label>
            <McTextField id="sort" v-model="form.sort" inputmode="numeric" />
          </div>
        </div>

        <fieldset class="field">
          <legend class="label">Limit per player</legend>
          <label class="check">
            <input v-model="form.limitOn" type="checkbox" />
            <span>Limit how many sets one player can order over time</span>
          </label>
          <div v-if="form.limitOn" class="cols">
            <div class="field">
              <label for="plimit" class="label">Sets per player</label>
              <McTextField id="plimit" v-model="form.playerLimit" inputmode="numeric" :invalid="!!errors.playerLimit" />
              <p v-if="errors.playerLimit" class="error">{{ errors.playerLimit }}</p>
            </div>
            <div class="field">
              <label for="pdays" class="label">Within days</label>
              <McTextField id="pdays" v-model="form.playerLimitDays" inputmode="numeric" :invalid="!!errors.playerLimitDays" />
              <p v-if="errors.playerLimitDays" class="error">{{ errors.playerLimitDays }}</p>
            </div>
          </div>
        </fieldset>

        <label class="check">
          <input v-model="form.enabled" type="checkbox" />
          <span>Show in the shop</span>
        </label>
        <label class="check">
          <input v-model="form.inStock" type="checkbox" :disabled="!form.enabled" />
          <span>In stock (when off, players see the item greyed out and cannot order it)</span>
        </label>

        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <p v-if="saved" class="ok" role="status">{{ saved }}</p>

        <div class="actions">
          <McButton v-if="!isNew" @click="remove">Remove item</McButton>
          <McButton type="submit" :disabled="saving">{{ saving ? 'Saving...' : isNew ? 'Add item' : 'Save changes' }}</McButton>
        </div>
      </template>
    </form>
  </AdminFrame>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 6);
  max-width: calc(var(--u) * 260);
  margin: 0 auto;
}

.head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.preview {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 4);
}

.field {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}

.label {
  color: var(--title);
  padding: 0;
}

.cols {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: calc(var(--u) * 4);
}

.cols.three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.currency {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 2);
}

.select {
  min-height: calc(var(--u) * 20);
  padding: 0 calc(var(--u) * 3);
  border: var(--u) solid #a0a0a0;
  background: #000;
  color: #e0e0e0;
}

.check {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 3);
  color: var(--title);
  cursor: pointer;
}

.check input {
  width: calc(var(--u) * 10);
  height: calc(var(--u) * 10);
  accent-color: #555;
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

.actions {
  display: flex;
  justify-content: space-between;
  gap: calc(var(--u) * 2);
}

@media (max-width: 699px) {
  .cols,
  .cols.three {
    grid-template-columns: 1fr;
  }
}
</style>
