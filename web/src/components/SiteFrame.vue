<script setup lang="ts">
import { useRoute } from 'vue-router'
import TitleLogo from './TitleLogo.vue'
import McButton from './McButton.vue'
import { cartUnits } from '../stores/cart'
import { soundOn } from '../stores/sound'
import { session } from '../stores/session'
import { assets } from '../stores/assets'

defineProps<{ wide?: boolean }>()
const route = useRoute()
</script>

<template>
  <div class="frame">
    <header class="header">
      <TitleLogo />
      <nav class="nav" aria-label="Shop">
        <McButton to="/" :active="route.path === '/'">Shop</McButton>
        <McButton to="/cart" :active="route.path === '/cart' || route.path === '/checkout'">
          Cart{{ cartUnits ? ` (${cartUnits})` : '' }}
        </McButton>
        <McButton to="/receipts" :active="route.path.startsWith('/receipt')">Receipts</McButton>
        <McButton v-if="session.admin" to="/admin" :active="route.path.startsWith('/admin')">Admin</McButton>
      </nav>
    </header>

    <main class="main" :class="{ wide }">
      <slot />
    </main>

    <footer class="footer shadow">
      <span>Hamazon{{ assets.version ? ` for ${assets.version}` : '' }}</span>
      <McButton small :aria-pressed="soundOn" @click="soundOn = !soundOn">Sound: {{ soundOn ? 'ON' : 'OFF' }}</McButton>
      <span>Not an official Minecraft product</span>
    </footer>
  </div>
</template>

<style scoped>
.frame {
  overflow-x: clip;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--u) * 6);
  padding: calc(var(--u) * 6) calc(var(--u) * 4) calc(var(--u) * 8);
}

.nav {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: calc(var(--u) * 2);
}

.main {
  flex: 1;
  width: 100%;
  max-width: calc(var(--u) * 280 + 16px);
  margin: 0 auto;
  padding: 0 8px calc(var(--u) * 12);
}

.main.wide {
  max-width: calc(var(--u) * 400);
}

.footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--u) * 4);
  padding: calc(var(--u) * 2) calc(var(--u) * 2);
  color: var(--text);
}
</style>
