import { reactive, watch } from 'vue'
import { readJSON, writeJSON } from '../lib/storage'

const KEY = 'hamazon.receipts.v1'

export interface SavedReceipt {
  token: string
  code: string
  username: string
  createdAt: string
}

export const receipts = reactive<{ list: SavedReceipt[] }>({ list: readJSON<SavedReceipt[]>(KEY, []) })

watch(
  () => receipts.list,
  (list) => writeJSON(KEY, list),
  { deep: true },
)

export function rememberReceipt(r: SavedReceipt) {
  receipts.list = [r, ...receipts.list.filter((x) => x.token !== r.token)].slice(0, 50)
}

export function forgetReceipt(token: string) {
  receipts.list = receipts.list.filter((x) => x.token !== token)
}
