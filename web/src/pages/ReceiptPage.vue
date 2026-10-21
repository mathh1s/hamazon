<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import SiteFrame from '../components/SiteFrame.vue'
import ItemIcon from '../components/ItemIcon.vue'
import PriceTag from '../components/PriceTag.vue'
import McButton from '../components/McButton.vue'
import { api, ApiError, errorMessage, type Receipt } from '../api'
import { loadCatalog } from '../stores/catalog'
import { rememberReceipt } from '../stores/receipts'
import { formatDate, statusLabel } from '../lib/format'
import { scalePrice } from '../lib/price'
import { usePageTitle } from '../lib/title'

const route = useRoute()
const token = computed(() => String(route.params.token ?? ''))
const justPlaced = computed(() => route.query.placed === '1')

const receipt = ref<Receipt | null>(null)
const error = ref('')
const notFound = ref(false)
const copied = ref(false)

usePageTitle(() => (receipt.value ? `Receipt ${receipt.value.code}` : 'Receipt'))

async function load() {
  error.value = ''
  try {
    await loadCatalog()
    receipt.value = await api<Receipt>('GET', `/api/receipts/${encodeURIComponent(token.value)}`)
    rememberReceipt({ token: token.value, code: receipt.value.code, username: receipt.value.username, createdAt: receipt.value.createdAt })
  } catch (e) {
    notFound.value = e instanceof ApiError && e.status === 404
    error.value = errorMessage(e)
  }
}

onMounted(load)

function print() {
  window.print()
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(location.origin + `/receipt/${token.value}`)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    copied.value = false
  }
}
</script>

<template>
  <SiteFrame>
    <div class="receipt-page">
      <p v-if="!receipt && !error" class="state shadow">Opening receipt...</p>

      <div v-else-if="error" class="state">
        <p class="shadow">{{ error }}</p>
        <McButton v-if="!notFound" @click="load">Try again</McButton>
        <McButton to="/receipts">Your receipts</McButton>
      </div>

      <template v-else-if="receipt">
        <article class="book" aria-labelledby="receipt-title">
          <header class="top">
            <h2 id="receipt-title">Hamazon Receipt</h2>
            <p v-if="justPlaced" class="placed">Order placed! The admins have your request.</p>
          </header>

          <dl class="facts">
            <dt>Order</dt>
            <dd>{{ receipt.code }}</dd>
            <dt>Player</dt>
            <dd>{{ receipt.username }}</dd>
            <dt>Placed</dt>
            <dd>{{ formatDate(receipt.createdAt) }}</dd>
            <dt>Status</dt>
            <dd :class="'status-' + receipt.status">{{ statusLabel[receipt.status] }}</dd>
          </dl>

          <ul class="lines">
            <li v-for="(line, i) in receipt.lines" :key="i">
              <ItemIcon :texture="line.texture" :size="12" />
              <span class="name">
                {{ line.quantity }} × {{ line.name }}<template v-if="line.count > 1"> ({{ line.quantity * line.count }})</template>
                <span class="each"><PriceTag :price="line.unitPrice" :icon-size="7" /><span>each</span></span>
              </span>
              <PriceTag :price="scalePrice(line.unitPrice, line.quantity)" :icon-size="7" />
            </li>
          </ul>

          <div class="total">
            <span>Total</span>
            <PriceTag :price="receipt.totals" stacked :icon-size="8" />
          </div>

          <p v-if="receipt.note" class="note">Your note: {{ receipt.note }}</p>
          <p class="foot">Pay in game and show this order id to an admin. Items are handed over by hand.</p>
        </article>

        <div class="actions">
          <McButton @click="print">Print or save as PDF</McButton>
          <McButton @click="copyLink">{{ copied ? 'Link copied' : 'Copy link' }}</McButton>
          <McButton to="/">Back to shop</McButton>
        </div>
        <p class="private shadow">Anyone with this link can see the receipt, so only share it with admins.</p>
      </template>
    </div>
  </SiteFrame>
</template>

<style scoped>
.receipt-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--u) * 6);
}

.state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--u) * 3);
}

.state p {
  margin: 0;
}

/* Book and quill page, nine-sliced so the rings repeat down a long order. */
.book {
  width: min(100%, calc(var(--u) * 180));
  border-style: solid;
  border-color: transparent;
  border-width: calc(var(--u) * 10) calc(var(--u) * 12) calc(var(--u) * 12) calc(var(--u) * 16);
  border-image: url('/mc/gui/book.png') 10 12 12 16 fill / calc(var(--u) * 10) calc(var(--u) * 12) calc(var(--u) * 12)
    calc(var(--u) * 16) round;
  padding: calc(var(--u) * 2) calc(var(--u) * 2) calc(var(--u) * 4) calc(var(--u) * 2);
  color: #000;
}

h2 {
  margin: 0;
  font-size: var(--font-size);
  font-weight: normal;
  text-align: center;
}

.placed {
  margin: calc(var(--u) * 2) 0 0;
  color: #00aa00;
  text-align: center;
}

.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: var(--u) calc(var(--u) * 6);
  margin: calc(var(--u) * 6) 0;
}

.facts dt {
  color: #555;
}

.facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.status-pending {
  color: #0000aa;
}

.status-approved {
  color: #00aaaa;
}

.status-fulfilled {
  color: #00aa00;
}

.status-cancelled {
  color: #aa0000;
}

.lines {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 3);
}

.lines li {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: calc(var(--u) * 3);
  align-items: start;
}

.name {
  display: flex;
  flex-direction: column;
}

.each {
  display: inline-flex;
  align-items: center;
  gap: var(--u);
  color: #555;
}

.total {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-top: calc(var(--u) * 6);
  padding-top: calc(var(--u) * 3);
  border-top: var(--u) dashed #9a8a6a;
}

.note,
.foot {
  margin: calc(var(--u) * 6) 0 0;
  color: #555;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: calc(var(--u) * 2);
}

.private {
  margin: 0;
  color: var(--gray);
  text-align: center;
}

@media print {
  :global(.header),
  :global(.footer),
  .actions,
  .private {
    display: none !important;
  }

  .book {
    print-color-adjust: exact;
    -webkit-print-color-adjust: exact;
    width: 100%;
  }
}
</style>
