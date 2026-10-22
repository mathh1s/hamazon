<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { assets } from '../stores/assets'

const props = withDefaults(defineProps<{ texture: string; size?: number; alt?: string }>(), { size: 16, alt: '' })

// Items the game always draws with the enchantment glint.
const GLINT = new Set(['enchanted_golden_apple', 'nether_star', 'experience_bottle', 'enchanted_book', 'end_crystal', 'written_book'])

// The game's own missing texture: magenta and black squares.
const MISSING =
  'data:image/svg+xml,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 2" shape-rendering="crispEdges"><rect width="2" height="2" fill="#000"/><rect width="1" height="1" fill="#f800f8"/><rect x="1" y="1" width="1" height="1" fill="#f800f8"/></svg>',
  )

const broken = ref(false)
watch(
  () => [props.texture, assets.stamp],
  () => (broken.value = false),
)

// The stamp changes with every extraction, so a new Minecraft version never
// shows icons from the browser cache.
const src = computed(() =>
  broken.value ? MISSING : `/mc/item/${props.texture}.png${assets.stamp ? `?v=${encodeURIComponent(assets.stamp)}` : ''}`,
)
const kind = computed(() => assets.items.get(props.texture)?.kind ?? 'flat')
const glint = computed(() => !broken.value && GLINT.has(props.texture))
</script>

<template>
  <span class="icon" :class="{ glint }" :style="{ '--size': size, '--icon': `url(${src})` }">
    <img :src="src" :alt="alt" :class="kind" draggable="false" loading="lazy" @error="broken = true" />
  </span>
</template>

<style scoped>
.icon {
  position: relative;
  display: inline-block;
  width: calc(var(--u) * var(--size));
  height: calc(var(--u) * var(--size));
  flex: none;
}

img {
  display: block;
  width: 100%;
  height: 100%;
}

/* Block icons are pre-rendered at high resolution, smooth scaling keeps
   their diagonal edges clean. Flat sprites stay sharp. */
img.block {
  image-rendering: auto;
}

.glint::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  -webkit-mask: var(--icon) center / 100% 100% no-repeat;
  mask: var(--icon) center / 100% 100% no-repeat;
  background: linear-gradient(115deg, transparent 30%, rgba(170, 110, 255, 0.55) 45%, transparent 60%) 0 0 / 300% 300%;
  mix-blend-mode: screen;
  animation: glint 3s linear infinite;
}

@keyframes glint {
  from {
    background-position: 100% 100%;
  }
  to {
    background-position: 0 0;
  }
}
</style>
