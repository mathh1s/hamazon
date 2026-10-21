<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import SiteFrame from '../components/SiteFrame.vue'
import McButton from '../components/McButton.vue'
import McTextField from '../components/McTextField.vue'
import { forgetReceipt, receipts } from '../stores/receipts'
import { formatDate } from '../lib/format'
import { usePageTitle } from '../lib/title'

usePageTitle(() => 'Receipts')
const router = useRouter()
const link = ref('')
const linkError = ref('')

function open() {
  const raw = link.value.trim()
  const token = raw.includes('/receipt/') ? raw.split('/receipt/')[1]!.split(/[?#]/)[0]! : raw
  if (!/^[A-Za-z0-9_-]{20,64}$/.test(token)) {
    linkError.value = 'Paste the full receipt link you got after ordering'
    return
  }
  linkError.value = ''
  router.push(`/receipt/${token}`)
}
</script>

<template>
  <SiteFrame>
    <section class="mc-panel receipts" aria-labelledby="receipts-title">
      <h2 id="receipts-title" class="mc-title">Your receipts</h2>
      <p v-if="receipts.list.length === 0" class="hint">
        No receipts on this device yet. Receipts are saved here after you place an order.
      </p>
      <ul v-else class="list">
        <li v-for="r in receipts.list" :key="r.token">
          <RouterLink :to="`/receipt/${r.token}`" class="code">{{ r.code }}</RouterLink>
          <span>{{ r.username }}</span>
          <span class="date">{{ formatDate(r.createdAt) }}</span>
          <McButton small :aria-label="`Forget receipt ${r.code} on this device`" @click="forgetReceipt(r.token)">Forget</McButton>
        </li>
      </ul>

      <form class="open" @submit.prevent="open">
        <label for="link">Open a receipt link</label>
        <div class="row">
          <McTextField id="link" v-model="link" :invalid="!!linkError" placeholder="https://.../receipt/..." />
          <McButton type="submit">Open</McButton>
        </div>
        <p v-if="linkError" class="error">{{ linkError }}</p>
      </form>
    </section>
  </SiteFrame>
</template>

<style scoped>
.receipts {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 6);
}

.hint {
  margin: 0;
  color: var(--dark-gray);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.list li {
  display: grid;
  grid-template-columns: auto 1fr auto auto;
  align-items: center;
  gap: calc(var(--u) * 4);
}

.code {
  color: #0000aa;
}

.date {
  color: var(--dark-gray);
}

.open {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.row {
  display: flex;
  gap: calc(var(--u) * 2);
}

.error {
  margin: 0;
  color: #aa0000;
}

@media (max-width: 899px) {
  .list li {
    grid-template-columns: 1fr auto;
  }
}
</style>
