import { createRouter, createWebHistory } from 'vue-router'
import { loadSession, session } from './stores/session'
import { setAccessRequiredHandler } from './api'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/gate', component: () => import('./pages/GatePage.vue'), meta: { public: true } },
    { path: '/', component: () => import('./pages/ShopPage.vue') },
    { path: '/cart', component: () => import('./pages/CartPage.vue') },
    { path: '/checkout', component: () => import('./pages/CheckoutPage.vue') },
    { path: '/receipts', component: () => import('./pages/ReceiptsPage.vue') },
    { path: '/receipt/:token', component: () => import('./pages/ReceiptPage.vue') },
    { path: '/:pathMatch(.*)*', component: () => import('./pages/NotFoundPage.vue') },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  if (!session.loaded) {
    try {
      await loadSession()
    } catch {
      // Without a session answer the gate is the safe place to land.
    }
  }
  if (!to.meta.public && !session.access) {
    return { path: '/gate', query: to.fullPath !== '/' ? { next: to.fullPath } : {} }
  }
  if (to.path === '/gate' && session.access) return '/'
})

setAccessRequiredHandler(() => {
  session.access = false
  const current = router.currentRoute.value
  if (!current.meta.public) router.replace({ path: '/gate', query: { next: current.fullPath } })
})
