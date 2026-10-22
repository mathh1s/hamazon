<script setup lang="ts">
import { watch } from 'vue'
import McTooltip from './components/McTooltip.vue'
import ToastStack from './components/ToastStack.vue'
import { loadAssets } from './stores/assets'
import { session } from './stores/session'
import { watchOrders } from './stores/orderWatch'
import { stopWatchingOrders, watchNewOrders } from './stores/adminWatch'

loadAssets()

watch(
  () => session.access,
  (ok) => ok && watchOrders(),
  { immediate: true },
)
watch(
  () => session.admin,
  (admin) => (admin ? watchNewOrders() : stopWatchingOrders()),
  { immediate: true },
)
</script>

<template>
  <RouterView />
  <McTooltip />
  <ToastStack />
</template>
