import { reactive, watch } from 'vue'
import { readJSON, writeJSON } from '../lib/storage'

export const HOTBAR_SIZE = 9
const KEY = 'hamazon.hotbar.v1'

export const favorites = reactive<{ ids: number[] }>({
  ids: readJSON<number[]>(KEY, [])
    .filter((n) => Number.isInteger(n))
    .slice(0, HOTBAR_SIZE),
})

watch(
  () => favorites.ids,
  (ids) => writeJSON(KEY, ids),
  { deep: true },
)

export function isFavorite(id: number) {
  return favorites.ids.includes(id)
}

export function toggleFavorite(id: number) {
  if (isFavorite(id)) {
    favorites.ids = favorites.ids.filter((x) => x !== id)
  } else if (favorites.ids.length < HOTBAR_SIZE) {
    favorites.ids = [...favorites.ids, id]
  }
}
