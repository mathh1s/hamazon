<script setup lang="ts">
import type { OrderStatus } from '../api'
import { statusLabel } from '../lib/format'

defineProps<{ status: OrderStatus }>()
</script>

<template>
  <span class="status shadow" :class="status">{{ statusLabel[status] }}</span>
</template>

<style scoped>
.status {
  color: var(--text);
}

.pending {
  color: var(--gold);
}

.approved {
  color: var(--aqua);
}

.fulfilled {
  color: var(--green);
}

.cancelled {
  color: var(--red);
}
</style>
