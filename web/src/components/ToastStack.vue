<script setup lang="ts">
import { useRouter } from 'vue-router'
import ItemIcon from './ItemIcon.vue'
import { dismissToast, toasts, type Toast } from '../stores/toasts'

const router = useRouter()

function open(t: Toast) {
  dismissToast(t.id)
  if (t.to) router.push(t.to)
}
</script>

<template>
  <div class="stack" aria-live="polite">
    <TransitionGroup name="toast">
      <button v-for="t in toasts.list" :key="t.id" type="button" class="toast" @click="open(t)">
        <ItemIcon :texture="t.icon" class="icon" />
        <span class="title">{{ t.title }}</span>
        <span class="text">{{ t.text }}</span>
      </button>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.stack {
  position: fixed;
  top: 0;
  right: 0;
  z-index: 90;
  display: flex;
  flex-direction: column;
  max-width: 100vw;
}

/* The advancement toast sprite, nine-sliced so it can be a bit wider than
   the game's 160 pixels without stretching its border. */
.toast {
  position: relative;
  width: calc(var(--u) * 200);
  max-width: 100vw;
  height: calc(var(--u) * 32);
  padding: 0;
  border: calc(var(--u) * 4) solid #000;
  border-image: url('/mc/gui/toast.png') 4 fill / calc(var(--u) * 4) stretch;
  background: #212121 padding-box;
  text-align: left;
  cursor: pointer;
}

.icon {
  position: absolute;
  left: calc(var(--u) * 4);
  top: calc(var(--u) * 4);
}

.title,
.text {
  position: absolute;
  left: calc(var(--u) * 26);
  right: calc(var(--u) * 2);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  line-height: calc(var(--u) * 9);
}

.title {
  top: calc(var(--u) * 2);
  color: var(--yellow);
}

.text {
  top: calc(var(--u) * 13);
  color: var(--text);
}

.toast-enter-active,
.toast-leave-active {
  transition: transform 0.4s ease;
}

.toast-enter-from,
.toast-leave-to {
  transform: translateX(100%);
}
</style>
