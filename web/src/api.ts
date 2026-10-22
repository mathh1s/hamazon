export type Price = Record<string, number>

export interface Currency {
  id: string
  name: string
  texture: string
  sort: number
}

export interface Category {
  id: string
  name: string
  icon: string
  sort: number
}

export interface Item {
  id: number
  name: string
  texture: string
  categoryId: string
  count: number
  description: string
  maxPerOrder: number
  playerLimit: number | null
  playerLimitDays: number | null
  enabled: boolean
  inStock: boolean
  priceLocked?: boolean
  dimensions: Dimension[]
  group: string
  sort: number
  price: Price
}

export type Dimension = 'nether' | 'end'

export interface Catalog {
  currencies: Currency[]
  categories: Category[]
  items: Item[]
  disabledDimensions: Dimension[]
}

export type OrderStatus = 'pending' | 'approved' | 'fulfilled' | 'cancelled'

export interface OrderLine {
  itemId: number | null
  name: string
  texture: string
  count: number
  quantity: number
  unitPrice: Price
}

export interface Receipt {
  code: string
  username: string
  note: string
  status: OrderStatus
  createdAt: string
  updatedAt: string
  lines: OrderLine[]
  totals: Price
}

export interface Session {
  access: boolean
  floodgatePrefix: string
  admin: { name: string; csrfToken: string } | null
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: Record<string, string> = {},
    readonly details: unknown = null,
  ) {
    super(message)
  }
}

let csrfToken = ''
let onAccessRequired: () => void = () => {}
let onAdminRequired: () => void = () => {}

export function setCsrfToken(token: string) {
  csrfToken = token
}

export function setAccessRequiredHandler(fn: () => void) {
  onAccessRequired = fn
}

export function setAdminRequiredHandler(fn: () => void) {
  onAdminRequired = fn
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (csrfToken && method !== 'GET') headers['X-CSRF-Token'] = csrfToken

  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, 'network', 'Could not reach the shop. Check your connection and try again.')
  }

  if (res.status === 204) return undefined as T
  let data: unknown = null
  try {
    data = await res.json()
  } catch {
    // Non-JSON error pages from a proxy are handled below.
  }
  if (!res.ok) {
    const err = (data as { error?: { code: string; message: string; fields?: Record<string, string>; details?: unknown } })?.error
    const apiErr = new ApiError(
      res.status,
      err?.code ?? 'http_' + res.status,
      err?.message ?? `The server answered with an error (${res.status}).`,
      err?.fields ?? {},
      err?.details ?? null,
    )
    if (apiErr.code === 'access_required') onAccessRequired()
    if (apiErr.code === 'admin_required') onAdminRequired()
    throw apiErr
  }
  return data as T
}

export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) return e.message
  return 'Something went wrong. Try again.'
}

export interface OrderEvent {
  status: OrderStatus
  adminName: string
  note: string
  createdAt: string
}

export interface DiscordNotification {
  id: number
  kind: 'order_created' | 'order_updated'
  state: 'pending' | 'sent' | 'failed'
  attempts: number
  lastError: string
  nextAttemptAt: string
  createdAt: string
}

export interface AdminOrderSummary extends Receipt {
  id: number
  failedNotifications: number
}

export interface AdminOrder extends Receipt {
  id: number
  clientIp: string
  discordSent: boolean
  events: OrderEvent[]
  notifications: DiscordNotification[]
}

export interface ItemInput {
  name: string
  texture: string
  categoryId: string
  count: number
  description: string
  maxPerOrder: number
  playerLimit: number | null
  playerLimitDays: number | null
  enabled: boolean
  inStock: boolean
  dimensions: Dimension[]
  group: string
  sort: number
  price: Price
}

export interface PriceTerms {
  count: number
  maxPerOrder: number
  playerLimit: number | null
  playerLimitDays: number | null
  price: Price
}

export interface PriceUpdate {
  itemId: number
  name: string
  texture: string
  locked: boolean
  current: PriceTerms
  default: PriceTerms
}
