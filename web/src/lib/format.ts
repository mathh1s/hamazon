import type { OrderStatus } from '../api'

export function formatDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

export const statusLabel: Record<OrderStatus, string> = {
  pending: 'Pending',
  approved: 'Approved',
  fulfilled: 'Fulfilled',
  cancelled: 'Cancelled',
}

export function plural(n: number, one: string, many: string) {
  return n === 1 ? one : many
}
