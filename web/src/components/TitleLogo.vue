<script setup lang="ts">
const SPLASHES = [
  'Pay in diamonds!',
  'Now accepting emeralds!',
  'Gold ingots welcome!',
  'Also try Amazon!',
  'No creepers in stock!',
  'Delivery by hand!',
  'Mined fresh daily!',
  'Hrmm!',
  'Free shipping*!',
  'Stacks of 64!',
]
const splash = SPLASHES[Math.floor(Math.random() * SPLASHES.length)]

defineProps<{ compact?: boolean }>()
</script>

<template>
  <RouterLink to="/" class="logo" :class="{ compact }" aria-label="Hamazon shop home">
    <span class="word" data-text="HAMAZON" aria-hidden="true">HAMAZON</span>
    <span class="splash" aria-hidden="true">{{ splash }}</span>
  </RouterLink>
</template>

<style scoped>
.logo {
  position: relative;
  isolation: isolate;
  display: inline-block;
  text-decoration: none;
  padding: calc(var(--u) * 4) calc(var(--u) * 6) calc(var(--u) * 8);
}

/* Stone filled letters with a stepped extrusion, like the title screen logo.
   The extrusion lives on a copy behind the text because a text shadow would
   paint over a background-clip fill. */
.word {
  position: relative;
  display: block;
  font-size: calc(var(--u) * 24);
  line-height: 1;
  letter-spacing: calc(var(--u) * 2);
  color: transparent;
  background: url('/mc/gui/stone.png') 0 0 / calc(var(--u) * 24);
  -webkit-background-clip: text;
  background-clip: text;
}

.word::before {
  content: attr(data-text);
  position: absolute;
  inset: 0;
  z-index: -1;
  color: #000;
  text-shadow:
    calc(var(--u) * -1) 0 0 #000,
    calc(var(--u) * 1) 0 0 #000,
    0 calc(var(--u) * -1) 0 #000,
    0 calc(var(--u) * 2) 0 #2e2e2e,
    0 calc(var(--u) * 4) 0 #262626,
    calc(var(--u) * 1) calc(var(--u) * 5) 0 #000,
    calc(var(--u) * -1) calc(var(--u) * 5) 0 #000;
}

.compact .word {
  font-size: calc(var(--u) * 16);
  letter-spacing: var(--u);
}

.splash {
  position: absolute;
  right: calc(var(--u) * 2);
  top: calc(var(--u) * 22);
  color: var(--yellow);
  text-shadow: var(--u) var(--u) 0 #3f3f15;
  white-space: nowrap;
  transform: translate(50%, -50%) rotate(-20deg);
  transform-origin: center;
  animation: splash 0.5s ease-in-out infinite alternate;
}

@media (max-width: 599px) {
  .splash {
    right: calc(var(--u) * 24);
    top: calc(var(--u) * 26);
  }
}

.compact .splash {
  display: none;
}

@keyframes splash {
  from {
    scale: 1;
  }
  to {
    scale: 1.08;
  }
}
</style>
