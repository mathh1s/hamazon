import { reactive } from 'vue'

export interface Toast {
  id: number
  title: string
  text: string
  icon: string
  to?: string
}

export const toasts = reactive<{ list: Toast[] }>({ list: [] })
let next = 1

// Like the game, toasts slide in at the top right and leave on their own.
export function pushToast(t: Omit<Toast, 'id'>, ms = 7000) {
  const toast = { ...t, id: next++ }
  toasts.list = [...toasts.list.slice(-3), toast]
  setTimeout(() => dismissToast(toast.id), ms)
}

export function dismissToast(id: number) {
  toasts.list = toasts.list.filter((t) => t.id !== id)
}
