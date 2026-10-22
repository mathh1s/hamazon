<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import AdminFrame from '../../components/AdminFrame.vue'
import McButton from '../../components/McButton.vue'
import McTextField from '../../components/McTextField.vue'
import ItemSlot from '../../components/ItemSlot.vue'
import PriceTag from '../../components/PriceTag.vue'
import StatusText from '../../components/StatusText.vue'
import { api, errorMessage, type AdminOrder, type OrderStatus } from '../../api'
import { loadCatalog } from '../../stores/catalog'
import { loadOverview } from '../../stores/admin'
import { deliveryText, formatDate, statusLabel, teleportCommand } from '../../lib/format'
import { scalePrice } from '../../lib/price'
import { usePageTitle } from '../../lib/title'

const route = useRoute()
const id = computed(() => Number(route.params.id))
const order = ref<AdminOrder | null>(null)
const error = ref('')
const actionError = ref('')
const note = ref('')
const busy = ref(false)
const copied = ref(false)

async function copyTp() {
  if (!order.value?.delivery) return
  try {
    await navigator.clipboard.writeText(teleportCommand(order.value.delivery))
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // Clipboard can be blocked; the command is shown on the page anyway.
  }
}

usePageTitle(() => (order.value ? `Order ${order.value.code}` : 'Order'))

// Actions offered for each status, in the order an admin usually needs them.
const ACTIONS: Record<OrderStatus, { label: string; to: OrderStatus }[]> = {
  pending: [
    { label: 'Approve', to: 'approved' },
    { label: 'Mark fulfilled', to: 'fulfilled' },
    { label: 'Cancel order', to: 'cancelled' },
  ],
  approved: [
    { label: 'Mark fulfilled', to: 'fulfilled' },
    { label: 'Back to pending', to: 'pending' },
    { label: 'Cancel order', to: 'cancelled' },
  ],
  fulfilled: [{ label: 'Back to approved', to: 'approved' }],
  cancelled: [{ label: 'Reopen as pending', to: 'pending' }],
}

const failed = computed(() => order.value?.notifications.filter((n) => n.state === 'failed') ?? [])
const pending = computed(() => order.value?.notifications.filter((n) => n.state === 'pending') ?? [])

async function load() {
  error.value = ''
  try {
    order.value = await api<AdminOrder>('GET', `/api/admin/orders/${id.value}`)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function setStatus(to: OrderStatus) {
  if (to === 'cancelled' && !confirm('Cancel this order? The player can order these items again afterwards.')) return
  busy.value = true
  actionError.value = ''
  try {
    order.value = await api<AdminOrder>('PATCH', `/api/admin/orders/${id.value}`, { status: to, note: note.value })
    note.value = ''
  } catch (e) {
    actionError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function retry() {
  busy.value = true
  actionError.value = ''
  try {
    order.value = await api<AdminOrder>('POST', `/api/admin/orders/${id.value}/retry`)
    await loadOverview()
    setTimeout(load, 2500)
  } catch (e) {
    actionError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

loadCatalog()
load()
</script>

<template>
  <AdminFrame>
    <p v-if="error" class="shadow">{{ error }}</p>
    <p v-else-if="!order" class="shadow">Loading order...</p>
    <div v-else class="layout">
      <section class="mc-panel main" aria-labelledby="order-title">
        <div class="head">
          <h2 id="order-title" class="mc-title">Order {{ order.code }}</h2>
          <StatusText :status="order.status" class="badge" />
        </div>
        <dl class="facts">
          <dt>Player</dt>
          <dd>{{ order.username }}</dd>
          <dt>Placed</dt>
          <dd>{{ formatDate(order.createdAt) }}</dd>
          <dt>IP</dt>
          <dd>{{ order.clientIp || 'unknown' }}</dd>
          <dt>Deliver to</dt>
          <dd class="deliver">
            <span>{{ deliveryText(order.delivery) }}</span>
            <template v-if="order.delivery">
              <code class="tp">{{ teleportCommand(order.delivery) }}</code>
              <McButton small type="button" @click="copyTp">{{ copied ? 'Copied!' : 'Copy tp command' }}</McButton>
            </template>
          </dd>
          <template v-if="order.note">
            <dt>Note</dt>
            <dd>{{ order.note }}</dd>
          </template>
        </dl>

        <ul class="lines">
          <li v-for="(line, i) in order.lines" :key="i">
            <ItemSlot :texture="line.texture" :count="line.count" />
            <span class="name">
              <span>{{ line.quantity * line.count }} × {{ line.name }}</span>
              <span class="sub">{{ line.quantity }} × {{ line.count }}, <PriceTag :price="line.unitPrice" :icon-size="7" /><span>each</span></span>
            </span>
            <PriceTag :price="scalePrice(line.unitPrice, line.quantity)" />
          </li>
        </ul>
        <div class="total">
          <span>Player pays</span>
          <PriceTag :price="order.totals" stacked :icon-size="10" />
        </div>
      </section>

      <aside class="side">
        <section class="mc-panel" aria-labelledby="status-title">
          <h3 id="status-title" class="mc-title">Update status</h3>
          <label for="status-note" class="label">Note (shown in Discord, optional)</label>
          <McTextField id="status-note" v-model="note" maxlength="200" placeholder="e.g. paid in full" />
          <div class="actions">
            <McButton v-for="a in ACTIONS[order.status]" :key="a.to" :disabled="busy" @click="setStatus(a.to)">{{ a.label }}</McButton>
          </div>
          <p v-if="actionError" class="error" role="alert">{{ actionError }}</p>
        </section>

        <section class="mc-panel" aria-labelledby="discord-title">
          <h3 id="discord-title" class="mc-title">Discord</h3>
          <p class="hint">
            <template v-if="failed.length">{{ failed.length }} message {{ failed.length === 1 ? 'update' : 'updates' }} failed.</template>
            <template v-else-if="pending.length">Sending...</template>
            <template v-else-if="order.discordSent">Message posted and up to date.</template>
            <template v-else>No message posted yet.</template>
          </p>
          <ul v-if="failed.length" class="errors">
            <li v-for="n in failed" :key="n.id">{{ n.lastError }}</li>
          </ul>
          <McButton v-if="failed.length" :disabled="busy" @click="retry">Retry Discord message</McButton>
        </section>

        <section class="mc-panel" aria-labelledby="history-title">
          <h3 id="history-title" class="mc-title">History</h3>
          <ol class="events">
            <li v-for="(e, i) in order.events" :key="i">
              <span>{{ statusLabel[e.status] }}{{ e.adminName ? ` by ${e.adminName}` : '' }}</span>
              <span class="hint">{{ formatDate(e.createdAt) }}</span>
              <span v-if="e.note" class="hint">{{ e.note }}</span>
            </li>
          </ol>
        </section>
      </aside>
    </div>
  </AdminFrame>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
  gap: calc(var(--u) * 6);
  align-items: start;
}

.main,
.side > section {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 4);
}

.side {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 6);
}

.head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: calc(var(--u) * 2);
}

.badge {
  padding: var(--u) calc(var(--u) * 3);
  background: var(--slot-dark);
}

.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: var(--u) calc(var(--u) * 6);
  margin: 0;
}

.facts dt {
  color: var(--dark-gray);
}

.facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.lines {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.lines li {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: calc(var(--u) * 4);
  align-items: center;
}

.name {
  display: flex;
  flex-direction: column;
}

.sub {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: calc(var(--u) * 2);
  color: var(--dark-gray);
}

.total {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding-top: calc(var(--u) * 3);
  border-top: var(--u) solid var(--panel-dark);
}

.label,
.hint {
  color: var(--dark-gray);
  margin: 0;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: calc(var(--u) * 2);
}

.error,
.errors {
  margin: 0;
  color: #aa0000;
}

.errors {
  padding-left: calc(var(--u) * 6);
  overflow-wrap: anywhere;
}

.events {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 3);
}

.events li {
  display: flex;
  flex-direction: column;
}

@media (max-width: 1099px) {
  .layout {
    grid-template-columns: 1fr;
  }
}
.deliver {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: calc(var(--u) * 2);
}

.tp {
  font-family: inherit;
  word-break: break-all;
  color: var(--gray);
}
</style>
