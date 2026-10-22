import { ref } from 'vue'

export const notifySupported = typeof window !== 'undefined' && 'Notification' in window
export const notifyPermission = ref<NotificationPermission>(notifySupported ? Notification.permission : 'denied')

export async function requestNotify() {
  if (!notifySupported) return
  notifyPermission.value = await Notification.requestPermission()
}

// System notifications only make sense when the tab is in the background;
// in the foreground the in-page toast already shows the same thing.
export function browserNotify(title: string, body: string, icon: string, onClick?: () => void) {
  if (!notifySupported || notifyPermission.value !== 'granted' || !document.hidden) return
  try {
    const n = new Notification(title, { body, icon, tag: title + body })
    n.onclick = () => {
      window.focus()
      onClick?.()
      n.close()
    }
  } catch {
    // Some mobile browsers only allow notifications from a service worker.
  }
}
