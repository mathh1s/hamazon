<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import SiteFrame from '../components/SiteFrame.vue'
import ItemSlot from '../components/ItemSlot.vue'
import PriceTag from '../components/PriceTag.vue'
import McButton from '../components/McButton.vue'
import McTextField from '../components/McTextField.vue'
import { api, ApiError, errorMessage, type Receipt } from '../api'
import { catalog, loadCatalog } from '../stores/catalog'
import { cart, cartLines, cartProblems, cartTotals, clearCart } from '../stores/cart'
import { session } from '../stores/session'
import { rememberReceipt } from '../stores/receipts'
import { playLevelUp } from '../stores/sound'
import { readJSON, writeJSON } from '../lib/storage'
import { randomId } from '../lib/random'
import { scalePrice } from '../lib/price'
import { usePageTitle } from '../lib/title'

usePageTitle(() => 'Checkout')
const router = useRouter()

const username = ref(readJSON<string>('hamazon.username', ''))
const note = ref('')
const fieldErrors = ref<Record<string, string>>({})
const error = ref('')
const placing = ref(false)

// One id per checkout visit: a retried submit (double click, lost response)
// returns the same order instead of creating a second one.
const checkoutId = randomId()

onMounted(async () => {
  await loadCatalog(true)
  if (cart.entries.length === 0 || cartProblems.value.length) router.replace('/cart')
})

const javaPattern = /^[A-Za-z0-9_]{3,16}$/
const usernameHint = computed(() => {
  const p = session.floodgatePrefix
  return p ? `Bedrock players: start with "${p}", like ${p}Your_Gamertag` : ''
})

function checkUsername(): string {
  const name = username.value.trim()
  if (!name) return 'Enter your Minecraft username'
  const p = session.floodgatePrefix
  if (p && name.startsWith(p)) {
    const rest = name.slice(p.length).replace(/ /g, '_')
    if (rest && p.length + rest.length <= 16 && /^[A-Za-z0-9_]+$/.test(rest)) return ''
    return 'That is not a valid Bedrock name'
  }
  return javaPattern.test(name) ? '' : 'Usernames are 3 to 16 letters, numbers or underscores'
}

watch(username, () => {
  if (fieldErrors.value.username) fieldErrors.value = { ...fieldErrors.value, username: '' }
})

async function place() {
  if (placing.value) return
  error.value = ''
  const usernameError = checkUsername()
  fieldErrors.value = usernameError ? { username: usernameError } : {}
  if (usernameError) return

  placing.value = true
  try {
    const res = await api<{ receiptToken: string; receipt: Receipt }>('POST', '/api/orders', {
      idempotencyKey: checkoutId,
      username: username.value.trim(),
      note: note.value,
      lines: cart.entries.map((e) => ({ itemId: e.itemId, quantity: e.quantity, unitPrice: e.acceptedPrice })),
    })
    writeJSON('hamazon.username', username.value.trim())
    rememberReceipt({
      token: res.receiptToken,
      code: res.receipt.code,
      username: res.receipt.username,
      createdAt: res.receipt.createdAt,
      status: res.receipt.status,
    })
    clearCart()
    playLevelUp()
    router.replace({ path: `/receipt/${res.receiptToken}`, query: { placed: '1' } })
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.code === 'price_changed' || e.code === 'unavailable' || e.code === 'out_of_stock') {
        await loadCatalog(true)
        router.replace('/cart')
        return
      }
      fieldErrors.value = e.fields
    }
    error.value = errorMessage(e)
  } finally {
    placing.value = false
  }
}
</script>

<template>
  <SiteFrame>
    <form class="mc-panel checkout" novalidate @submit.prevent="place">
      <h2 class="mc-title">Checkout</h2>

      <div class="field">
        <label for="username">Minecraft username</label>
        <McTextField
          id="username"
          v-model="username"
          autocomplete="username"
          autocapitalize="off"
          spellcheck="false"
          maxlength="17"
          required
          :invalid="!!fieldErrors.username"
          :aria-describedby="fieldErrors.username ? 'username-error' : 'username-help'"
        />
        <p v-if="fieldErrors.username" id="username-error" class="error">{{ fieldErrors.username }}</p>
        <p v-else id="username-help" class="help">{{ usernameHint }}</p>
      </div>

      <div class="field">
        <label for="note">Note for the admins (optional)</label>
        <McTextField
          id="note"
          v-model="note"
          maxlength="200"
          placeholder="e.g. I am online most evenings"
          :invalid="!!fieldErrors.note"
        />
        <p v-if="fieldErrors.note" class="error">{{ fieldErrors.note }}</p>
      </div>

      <section aria-labelledby="summary-title">
        <h3 id="summary-title" class="mc-title">Your order</h3>
        <ul class="lines">
          <li v-for="line in cartLines" :key="line.entry.itemId">
            <template v-if="line.item">
              <ItemSlot :texture="line.item.texture" :count="line.item.count" />
              <span class="name">
                {{ line.entry.quantity }} × {{ line.item.name }}
                <span class="unit">
                  <PriceTag :price="line.entry.acceptedPrice" /><span>each</span>
                </span>
              </span>
              <PriceTag :price="scalePrice(line.entry.acceptedPrice, line.entry.quantity)" />
            </template>
          </li>
        </ul>
        <div class="sum">
          <span>Total to pay</span>
          <PriceTag :price="cartTotals" stacked :icon-size="10" />
        </div>
      </section>

      <p class="notice">
        Placing the order sends a purchase request to the server admins. Nothing is taken from or added to your
        inventory automatically: bring the payment in game and an admin hands over the items.
      </p>

      <p v-if="error" class="error" role="alert">{{ error }}</p>

      <div class="actions">
        <McButton to="/cart">Back to cart</McButton>
        <McButton type="submit" :disabled="placing || !catalog.loaded || cart.entries.length === 0">
          {{ placing ? 'Placing order...' : 'Place order' }}
        </McButton>
      </div>
    </form>
  </SiteFrame>
</template>

<style scoped>
.checkout {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 6);
}

.field {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.field p {
  margin: 0;
}

label {
  color: var(--title);
}

.help {
  color: var(--dark-gray);
  min-height: calc(var(--u) * 10);
}

.error {
  margin: 0;
  color: #aa0000;
}

.lines {
  list-style: none;
  margin: calc(var(--u) * 3) 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.lines li {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: calc(var(--u) * 4);
}

.name {
  display: flex;
  flex-direction: column;
  gap: var(--u);
}

.unit {
  display: inline-flex;
  align-items: center;
  gap: calc(var(--u) * 2);
  color: var(--dark-gray);
}

.sum {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding-top: calc(var(--u) * 3);
  border-top: var(--u) solid var(--panel-dark);
}

.notice {
  margin: 0;
  padding: calc(var(--u) * 3);
  background: var(--slot-dark);
  color: var(--gray);
  text-shadow: var(--u) var(--u) 0 #2a2a2a;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: calc(var(--u) * 2);
}
</style>
