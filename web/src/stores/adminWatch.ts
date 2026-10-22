import { ref, watch } from 'vue'
import { api } from '../api'
import { browserNotify } from '../lib/browserNotify'
import { readJSON, writeJSON } from '../lib/storage'
import { session } from './session'
import { forcePlay } from './sound'
import { pushToast } from './toasts'
import { router } from '../router'

interface NewOrder {
  id: number
  code: string
  username: string
  texture: string
}

export const unseenOrders = ref(0)
// Bumped on every new order so open order lists refresh themselves.
export const orderTick = ref(0)
export const alertSound = ref(readJSON<boolean>('hamazon.adminAlertSound', true))
watch(alertSound, (v) => writeJSON('hamazon.adminAlertSound', v))

let latest = 0
let timer: ReturnType<typeof setInterval> | undefined

async function poll() {
  if (!session.admin) return
  try {
    const res = await api<{ latestId: number; orders: NewOrder[] }>('GET', `/api/admin/orders/since?after=${latest}`)
    const first = latest === 0
    latest = Math.max(latest, res.latestId)
    if (first || res.orders.length === 0) return
    unseenOrders.value += res.orders.length
    orderTick.value++
    if (alertSound.value) forcePlay('levelup')
    for (const o of res.orders) {
      const to = `/admin/orders/${o.id}`
      pushToast({ title: 'New order!', text: `${o.code} from ${o.username}`, icon: o.texture || 'emerald', to }, 15000)
      browserNotify('Hamazon: new order', `${o.code} from ${o.username}`, `/mc/item/${o.texture || 'emerald'}.png`, () =>
        router.push(to),
      )
    }
  } catch {
    // Logged out or offline, the next tick tries again.
  }
}

export function watchNewOrders() {
  if (timer) return
  void poll()
  timer = setInterval(poll, 20_000)
}

export function stopWatchingOrders() {
  clearInterval(timer)
  timer = undefined
  latest = 0
  unseenOrders.value = 0
}
