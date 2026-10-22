import { watchEffect } from 'vue'
import { unseenOrders } from '../stores/adminWatch'

export function usePageTitle(title: () => string) {
  watchEffect(() => {
    const t = title()
    const prefix = unseenOrders.value > 0 ? `(${unseenOrders.value}) ` : ''
    document.title = prefix + (t ? `${t} - Hamazon` : 'Hamazon')
  })
}
