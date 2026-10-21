import { watchEffect } from 'vue'

export function usePageTitle(title: () => string) {
  watchEffect(() => {
    const t = title()
    document.title = t ? `${t} - Hamazon` : 'Hamazon'
  })
}
