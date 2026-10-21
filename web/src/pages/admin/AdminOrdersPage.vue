<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AdminFrame from '../../components/AdminFrame.vue'
import McButton from '../../components/McButton.vue'
import McTextField from '../../components/McTextField.vue'
import ItemIcon from '../../components/ItemIcon.vue'
import PriceTag from '../../components/PriceTag.vue'
import StatusText from '../../components/StatusText.vue'
import { api, errorMessage, type AdminOrderSummary, type OrderStatus } from '../../api'
import { loadCatalog } from '../../stores/catalog'
import { formatDate } from '../../lib/format'
import { usePageTitle } from '../../lib/title'

usePageTitle(() => 'Orders')
const route = useRoute()
const router = useRouter()

const FILTERS: { label: string; value: OrderStatus | '' }[] = [
  { label: 'All', value: '' },
  { label: 'Pending', value: 'pending' },
  { label: 'Approved', value: 'approved' },
  { label: 'Fulfilled', value: 'fulfilled' },
  { label: 'Cancelled', value: 'cancelled' },
]

const status = ref<OrderStatus | ''>((route.query.status as OrderStatus) ?? '')
const search = ref(typeof route.query.q === 'string' ? route.query.q : '')
const page = ref(Number(route.query.page) || 1)
const orders = ref<AdminOrderSummary[]>([])
const total = ref(0)
const pageSize = ref(25)
const loading = ref(false)
const error = ref('')

const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

let timer: ReturnType<typeof setTimeout> | undefined
async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (status.value) params.set('status', status.value)
    if (search.value.trim()) params.set('q', search.value.trim())
    params.set('page', String(page.value))
    const res = await api<{ orders: AdminOrderSummary[]; total: number; pageSize: number }>('GET', '/api/admin/orders?' + params)
    orders.value = res.orders
    total.value = res.total
    pageSize.value = res.pageSize
    router.replace({ query: Object.fromEntries(params) })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

loadCatalog()
load()
watch(status, () => {
  page.value = 1
  load()
})
watch(search, () => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    page.value = 1
    load()
  }, 300)
})
watch(page, load)
</script>

<template>
  <AdminFrame>
    <section class="mc-panel orders" aria-labelledby="orders-title">
      <div class="head">
        <h2 id="orders-title" class="mc-title">Orders</h2>
        <McButton small :disabled="loading" @click="load">Refresh</McButton>
      </div>
      <div class="filters">
        <McButton v-for="f in FILTERS" :key="f.value" small :active="status === f.value" @click="status = f.value">{{ f.label }}</McButton>
      </div>
      <McTextField v-model="search" type="search" placeholder="Search order id or player" aria-label="Search orders" />

      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-else-if="!loading && orders.length === 0" class="hint">
        {{ search || status ? 'No orders match these filters.' : 'No orders yet. They show up here as soon as a player checks out.' }}
      </p>

      <ul class="list" :aria-busy="loading">
        <li v-for="o in orders" :key="o.id">
          <RouterLink :to="`/admin/orders/${o.id}`" class="row">
            <span class="top">
              <span class="code">{{ o.code }}</span>
              <span class="player">{{ o.username }}</span>
              <span v-if="o.failedNotifications" class="failed">Discord failed</span>
              <StatusText :status="o.status" />
            </span>
            <span class="bottom">
              <span class="date">{{ formatDate(o.createdAt) }}</span>
              <span class="icons" aria-hidden="true">
                <ItemIcon v-for="(l, i) in o.lines.slice(0, 6)" :key="i" :texture="l.texture" :size="10" />
                <span v-if="o.lines.length > 6" class="more">+{{ o.lines.length - 6 }}</span>
              </span>
              <PriceTag :price="o.totals" :icon-size="7" class="sum" />
            </span>
          </RouterLink>
        </li>
      </ul>

      <div v-if="pages > 1" class="pager">
        <McButton small :disabled="page <= 1" @click="page--">Previous</McButton>
        <span>Page {{ page }} of {{ pages }}</span>
        <McButton small :disabled="page >= pages" @click="page++">Next</McButton>
      </div>
    </section>
  </AdminFrame>
</template>

<style scoped>
.orders {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 4);
}

.head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: calc(var(--u) * 2);
}

.hint {
  margin: 0;
  color: var(--dark-gray);
}

.error {
  margin: 0;
  color: #aa0000;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--u);
}

.row {
  display: flex;
  flex-direction: column;
  gap: var(--u);
  padding: calc(var(--u) * 2) calc(var(--u) * 3);
  background: var(--slot-dark);
  color: var(--text);
  text-decoration: none;
  text-shadow: var(--u) var(--u) 0 var(--text-shadow);
}

.top,
.bottom {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: calc(var(--u) * 2) calc(var(--u) * 6);
}

.player {
  flex: 1;
}

.sum {
  margin-left: auto;
}

.row:hover,
.row:focus-visible {
  background: #4a4a4a;
  outline: var(--u) solid var(--text);
}

.code {
  color: var(--yellow);
}

.date {
  color: var(--gray);
}

.icons {
  display: flex;
  align-items: center;
  gap: var(--u);
}

.more {
  color: var(--gray);
}

.failed {
  color: var(--red);
}

.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: calc(var(--u) * 4);
}
</style>
