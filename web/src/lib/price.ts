import type { Currency, Price } from '../api'

export function samePrice(a: Price, b: Price) {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  for (const k of keys) if ((a[k] ?? 0) !== (b[k] ?? 0)) return false
  return true
}

export function scalePrice(p: Price, n: number): Price {
  const out: Price = {}
  for (const [k, v] of Object.entries(p)) out[k] = v * n
  return out
}

export function addPrices(prices: Price[]): Price {
  const out: Price = {}
  for (const p of prices) for (const [k, v] of Object.entries(p)) out[k] = (out[k] ?? 0) + v
  return out
}

// priceParts orders amounts by the configured currency order.
export function priceParts(p: Price, currencies: Currency[]) {
  return currencies.filter((c) => (p[c.id] ?? 0) > 0).map((c) => ({ currency: c, amount: p[c.id]! }))
}

export function currencyLabel(c: Currency, amount: number) {
  return amount === 1 ? c.name : c.name + 's'
}

export function priceText(p: Price, currencies: Currency[]) {
  return priceParts(p, currencies)
    .map(({ currency, amount }) => `${amount} ${currencyLabel(currency, amount)}`)
    .join(' + ')
}
