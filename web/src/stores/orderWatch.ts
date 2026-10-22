import { api, type OrderStatus } from '../api'
import { browserNotify } from '../lib/browserNotify'
import { receipts } from './receipts'
import { session } from './session'
import { playOrb } from './sound'
import { pushToast } from './toasts'
import { router } from '../router'

// toast is short enough for the toast sprite, long is for system notifications.
const MESSAGES: Record<OrderStatus, { title: string; toast: (c: string) => string; long: (c: string) => string; icon: string }> = {
  approved: {
    title: 'Order approved!',
    toast: (c) => `${c}: pay in game`,
    long: (c) => `${c}: bring the payment to an admin`,
    icon: 'emerald',
  },
  fulfilled: {
    title: 'Order fulfilled!',
    toast: (c) => `${c} was handed over`,
    long: (c) => `${c}: your items were handed over`,
    icon: 'chest',
  },
  cancelled: {
    title: 'Order cancelled',
    toast: (c) => `${c} was cancelled`,
    long: (c) => `${c} was cancelled by an admin`,
    icon: 'barrier',
  },
  pending: {
    title: 'Order reopened',
    toast: (c) => `${c} is pending again`,
    long: (c) => `${c} is waiting for an admin again`,
    icon: 'clock',
  },
}

let timer: ReturnType<typeof setInterval> | undefined

async function check() {
  // Orders that are done cannot change any more in practice, skip them.
  const open = receipts.list.filter((r) => r.status !== 'fulfilled' && r.status !== 'cancelled').slice(0, 50)
  if (!session.access || open.length === 0) return
  try {
    const res = await api<{ receipts: { token: string; code: string; status: OrderStatus }[] }>('POST', '/api/receipts/status', {
      tokens: open.map((r) => r.token),
    })
    for (const s of res.receipts) {
      const saved = receipts.list.find((r) => r.token === s.token)
      if (!saved) continue
      // A receipt saved before status tracking existed just records its status.
      if (saved.status && saved.status !== s.status) {
        const m = MESSAGES[s.status]
        const to = `/receipt/${s.token}`
        pushToast({ title: m.title, text: m.toast(s.code), icon: m.icon, to })
        browserNotify(`Hamazon: ${m.title}`, m.long(s.code), `/mc/item/${m.icon}.png`, () => router.push(to))
        playOrb()
      }
      saved.status = s.status
    }
  } catch {
    // Offline or locked, try again on the next tick.
  }
}

export function watchOrders() {
  if (timer) return
  void check()
  timer = setInterval(check, 45_000)
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) void check()
  })
}
