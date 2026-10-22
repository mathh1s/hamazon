import { computed, reactive, watch } from 'vue'
import type { Item, Price } from '../api'
import { readJSON, writeJSON } from '../lib/storage'
import { addPrices, scalePrice, samePrice } from '../lib/price'
import { catalog, itemsById } from './catalog'

const KEY = 'hamazon.cart.v1'

// acceptedPrice is the price the player saw when adding the item. If the
// catalog changes it, the line is flagged until they accept the new price.
export interface CartEntry {
  itemId: number
  quantity: number
  acceptedPrice: Price
}

export const cart = reactive<{ entries: CartEntry[] }>({
  entries: readJSON<CartEntry[]>(KEY, []).filter(
    (e) => Number.isInteger(e.itemId) && Number.isInteger(e.quantity) && e.quantity > 0,
  ),
})

watch(
  () => cart.entries,
  (entries) => writeJSON(KEY, entries),
  { deep: true },
)

export interface CartLine {
  entry: CartEntry
  item: Item | undefined
  priceChanged: boolean
  overLimit: boolean
  outOfStock: boolean
}

export const cartLines = computed<CartLine[]>(() =>
  cart.entries.map((entry) => {
    const item = itemsById.value.get(entry.itemId)
    return {
      entry,
      item,
      priceChanged: !!item && !samePrice(item.price, entry.acceptedPrice),
      overLimit: !!item && entry.quantity > item.maxPerOrder,
      outOfStock: !!item && !item.inStock,
    }
  }),
)

export const cartTotals = computed(() =>
  addPrices(cartLines.value.filter((l) => l.item).map((l) => scalePrice(l.entry.acceptedPrice, l.entry.quantity))),
)

export const cartUnits = computed(() => cart.entries.reduce((n, e) => n + e.quantity, 0))

export const cartProblems = computed(() => {
  if (!catalog.loaded) return []
  const out: string[] = []
  if (cartLines.value.some((l) => !l.item)) out.push('unavailable')
  if (cartLines.value.some((l) => l.priceChanged)) out.push('price')
  if (cartLines.value.some((l) => l.overLimit)) out.push('limit')
  if (cartLines.value.some((l) => l.outOfStock)) out.push('stock')
  return out
})

export function quantityInCart(itemId: number) {
  return cart.entries.find((e) => e.itemId === itemId)?.quantity ?? 0
}

export function addToCart(item: Item, quantity: number) {
  const existing = cart.entries.find((e) => e.itemId === item.id)
  if (existing) {
    existing.quantity = Math.min(existing.quantity + quantity, item.maxPerOrder)
    existing.acceptedPrice = { ...item.price }
  } else {
    cart.entries.push({ itemId: item.id, quantity: Math.min(quantity, item.maxPerOrder), acceptedPrice: { ...item.price } })
  }
}

export function setQuantity(itemId: number, quantity: number) {
  const entry = cart.entries.find((e) => e.itemId === itemId)
  if (!entry) return
  const max = itemsById.value.get(itemId)?.maxPerOrder ?? quantity
  entry.quantity = Math.max(1, Math.min(Math.floor(quantity) || 1, max))
}

export function removeFromCart(itemId: number) {
  cart.entries = cart.entries.filter((e) => e.itemId !== itemId)
}

export function removeOutOfStock() {
  cart.entries = cart.entries.filter((e) => itemsById.value.get(e.itemId)?.inStock !== false)
}

export function acceptCurrentPrices() {
  cart.entries = cart.entries
    .filter((e) => itemsById.value.has(e.itemId))
    .map((e) => {
      const item = itemsById.value.get(e.itemId)!
      return { ...e, quantity: Math.min(e.quantity, item.maxPerOrder), acceptedPrice: { ...item.price } }
    })
}

export function clearCart() {
  cart.entries = []
}
