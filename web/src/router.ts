import { createRouter, createWebHistory } from 'vue-router'
import { loadSession, session } from './stores/session'
import { setAccessRequiredHandler, setAdminRequiredHandler } from './api'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/gate', component: () => import('./pages/GatePage.vue'), meta: { public: true } },
    { path: '/', component: () => import('./pages/ShopPage.vue') },
    { path: '/cart', component: () => import('./pages/CartPage.vue') },
    { path: '/checkout', component: () => import('./pages/CheckoutPage.vue') },
    { path: '/receipts', component: () => import('./pages/ReceiptsPage.vue') },
    { path: '/receipt/:token', component: () => import('./pages/ReceiptPage.vue') },
    { path: '/admin/login', component: () => import('./pages/admin/AdminLoginPage.vue'), meta: { public: true } },
    { path: '/admin', component: () => import('./pages/admin/AdminOrdersPage.vue'), meta: { admin: true } },
    { path: '/admin/orders/:id', component: () => import('./pages/admin/AdminOrderPage.vue'), meta: { admin: true } },
    { path: '/admin/items', component: () => import('./pages/admin/AdminItemsPage.vue'), meta: { admin: true } },
    { path: '/admin/prices', component: () => import('./pages/admin/AdminPricesPage.vue'), meta: { admin: true } },
    { path: '/admin/items/new', component: () => import('./pages/admin/AdminItemPage.vue'), meta: { admin: true } },
    { path: '/admin/items/:id', component: () => import('./pages/admin/AdminItemPage.vue'), meta: { admin: true } },
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
  // Admin pages have their own login so the shop password is not needed first.
  if (to.meta.admin && !session.admin) {
    return { path: '/admin/login', query: { next: to.fullPath } }
  }
  if (to.path === '/admin/login' && session.admin) return '/admin'
  if (!to.meta.public && !to.meta.admin && !session.access) {
    return { path: '/gate', query: to.fullPath !== '/' ? { next: to.fullPath } : {} }
  }
  if (to.path === '/gate' && session.access) return '/'
})

setAccessRequiredHandler(() => {
  session.access = false
  const current = router.currentRoute.value
  if (!current.meta.public) router.replace({ path: '/gate', query: { next: current.fullPath } })
})

setAdminRequiredHandler(() => {
  session.admin = null
  const current = router.currentRoute.value
  if (current.meta.admin) router.replace({ path: '/admin/login', query: { next: current.fullPath } })
})
