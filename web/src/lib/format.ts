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

export const dimensionLabel: Record<'overworld' | 'nether' | 'end', string> = {
  overworld: 'Overworld',
  nether: 'Nether',
  end: 'End',
}

// Text colors that match each dimension's look in the game.
export const dimensionColor: Record<'nether' | 'end', string> = {
  nether: '#ff5555',
  end: '#d78bff',
}

export function needsText(dims: ('nether' | 'end')[]) {
  if (dims.length === 0) return ''
  return 'Needs the ' + dims.map((d) => dimensionLabel[d]).join(' and the ')
}
