import { computed, reactive } from 'vue'
import { api, type Catalog, type Item } from '../api'

export const catalog = reactive<{ loaded: boolean; loading: boolean; error: string } & Catalog>({
  loaded: false,
  loading: false,
  error: '',
  currencies: [],
  categories: [],
  items: [],
})

export const itemsById = computed(() => new Map<number, Item>(catalog.items.map((i) => [i.id, i])))

export async function loadCatalog(force = false) {
  if ((catalog.loaded && !force) || catalog.loading) return
  catalog.loading = true
  catalog.error = ''
  try {
    const data = await api<Catalog>('GET', '/api/catalog')
    catalog.currencies = data.currencies
    catalog.categories = data.categories
    catalog.items = data.items
    catalog.loaded = true
  } catch (e) {
    catalog.error = e instanceof Error ? e.message : 'Could not load the shop'
  } finally {
    catalog.loading = false
  }
}
