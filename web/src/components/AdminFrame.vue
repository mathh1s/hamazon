<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TitleLogo from './TitleLogo.vue'
import McButton from './McButton.vue'
import { admin, adminLogout, loadOverview } from '../stores/admin'
import { assets } from '../stores/assets'
import { session } from '../stores/session'

const route = useRoute()
const router = useRouter()

onMounted(() => loadOverview().catch(() => {}))

async function logout() {
  await adminLogout().catch(() => {})
  router.replace('/admin/login')
}
</script>

<template>
  <div class="frame">
    <header class="header">
      <TitleLogo compact />
      <nav class="nav" aria-label="Admin">
        <McButton to="/admin" :active="route.path === '/admin' || route.path.startsWith('/admin/orders')">Orders</McButton>
        <McButton to="/admin/items" :active="route.path.startsWith('/admin/items')">Items</McButton>
        <McButton to="/">Shop</McButton>
        <McButton @click="logout">Log out {{ session.admin?.name }}</McButton>
      </nav>
    </header>

    <div class="warnings">
      <p v-if="admin.loaded && !admin.webhookConfigured" class="warn shadow">
        Discord notifications are off: set HAMAZON_DISCORD_WEBHOOK_URL in .env and restart.
      </p>
      <p v-if="admin.failedNotifications > 0" class="warn shadow">
        {{ admin.failedNotifications }} Discord {{ admin.failedNotifications === 1 ? 'message' : 'messages' }} failed. Open the
        order to retry.
      </p>
      <p v-if="assets.missing" class="warn shadow">
        Item textures are missing. Run "hamazon extract --jar &lt;client.jar&gt;" as described in the README.
      </p>
    </div>

    <main class="main">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.frame {
  min-height: 100vh;
  overflow-x: clip;
}

.header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--u) * 4);
  padding: calc(var(--u) * 4) 16px;
}

.nav {
  display: flex;
  flex-wrap: wrap;
  gap: calc(var(--u) * 2);
}

.warnings {
  max-width: calc(var(--u) * 360);
  margin: 0 auto;
  padding: 0 8px;
}

.warn {
  margin: 0 0 calc(var(--u) * 4);
  padding: calc(var(--u) * 3);
  background: rgba(0, 0, 0, 0.6);
  color: var(--gold);
}

.main {
  max-width: calc(var(--u) * 360);
  margin: 0 auto;
  padding: 0 8px calc(var(--u) * 12);
}
</style>
