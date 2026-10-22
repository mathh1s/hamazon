<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import type { DeliveryDimension, MapInfo } from '../api'
import { assets } from '../stores/assets'

const props = defineProps<{ info: MapInfo; dimension: DeliveryDimension; point: { x: number; z: number } | null }>()
const emit = defineEmits<{ pick: [{ x: number; z: number }] }>()

// One map pixel is drawn as two screen pixels so the biome edges stay crisp.
const PX = 2
const scales = computed(() => props.info.scales ?? [4, 16, 64])
const tile = computed(() => props.info.tileSize ?? 128)
const scaleIndex = ref(1)
const scale = computed(() => scales.value[scaleIndex.value]!)
const blocksPerPx = computed(() => scale.value / PX)

const el = ref<HTMLElement | null>(null)
const size = ref({ w: 600, h: 360 })
const center = ref({ x: props.point?.x ?? props.info.spawn?.x ?? 0, z: props.point?.z ?? props.info.spawn?.z ?? 0 })
const hover = ref<{ x: number; z: number } | null>(null)

let observer: ResizeObserver | undefined
onMounted(() => {
  observer = new ResizeObserver(([e]) => {
    if (e) size.value = { w: e.contentRect.width, h: e.contentRect.height }
  })
  if (el.value) observer.observe(el.value)
})
onUnmounted(() => observer?.disconnect())

watch(
  () => props.point,
  (p) => {
    if (p && !visible(p.x, p.z)) center.value = { ...p }
  },
)

function visible(x: number, z: number) {
  const s = toScreen(x, z)
  return s.left >= 0 && s.top >= 0 && s.left <= size.value.w && s.top <= size.value.h
}

function toScreen(x: number, z: number) {
  return {
    left: (x - center.value.x) / blocksPerPx.value + size.value.w / 2,
    top: (z - center.value.z) / blocksPerPx.value + size.value.h / 2,
  }
}

function toBlock(left: number, top: number) {
  return {
    x: Math.round(center.value.x + (left - size.value.w / 2) * blocksPerPx.value),
    z: Math.round(center.value.z + (top - size.value.h / 2) * blocksPerPx.value),
  }
}

const tiles = computed(() => {
  const span = tile.value * scale.value
  const x0 = center.value.x - (size.value.w / 2) * blocksPerPx.value
  const z0 = center.value.z - (size.value.h / 2) * blocksPerPx.value
  const x1 = x0 + size.value.w * blocksPerPx.value
  const z1 = z0 + size.value.h * blocksPerPx.value
  const out = []
  for (let tz = Math.floor(z0 / span); tz <= Math.floor(z1 / span); tz++) {
    for (let tx = Math.floor(x0 / span); tx <= Math.floor(x1 / span); tx++) {
      const s = toScreen(tx * span, tz * span)
      out.push({
        key: `${props.dimension}/${scale.value}/${tx}_${tz}`,
        src: `/api/map/${props.dimension}/${scale.value}/${tx}_${tz}.png?v=${props.info.version}`,
        left: s.left,
        top: s.top,
      })
    }
  }
  return out
})

// ==== villages, overworld only and only when zoomed in enough ====
interface Village {
  type: string
  x: number
  z: number
}
const villages = ref<Village[]>([])
let villageTimer: ReturnType<typeof setTimeout> | undefined
const VILLAGE_REGION = 4096

watch(
  [center, scale, () => props.dimension, size],
  () => {
    clearTimeout(villageTimer)
    if (props.dimension !== 'overworld' || scale.value > 16) {
      villages.value = []
      return
    }
    villageTimer = setTimeout(loadVillages, 250)
  },
  { immediate: true, deep: true },
)

const villageCache = new Map<string, Village[]>()
async function loadVillages() {
  const half = { w: (size.value.w / 2) * blocksPerPx.value, h: (size.value.h / 2) * blocksPerPx.value }
  const keys: string[] = []
  for (let rz = Math.floor((center.value.z - half.h) / VILLAGE_REGION); rz <= Math.floor((center.value.z + half.h) / VILLAGE_REGION); rz++)
    for (let rx = Math.floor((center.value.x - half.w) / VILLAGE_REGION); rx <= Math.floor((center.value.x + half.w) / VILLAGE_REGION); rx++)
      keys.push(`${rx}_${rz}`)
  await Promise.all(
    keys
      .filter((k) => !villageCache.has(k))
      .map(async (k) => {
        const [rx, rz] = k.split('_').map(Number) as [number, number]
        const x0 = rx * VILLAGE_REGION
        const z0 = rz * VILLAGE_REGION
        try {
          const res = await fetch(`/api/map/villages?x0=${x0}&z0=${z0}&x1=${x0 + VILLAGE_REGION - 1}&z1=${z0 + VILLAGE_REGION - 1}`)
          if (res.ok) villageCache.set(k, ((await res.json()) as { villages: Village[] }).villages)
        } catch {
          // Villages are a nice extra; the map works without them.
        }
      }),
  )
  villages.value = keys.flatMap((k) => villageCache.get(k) ?? [])
}

// ==== interaction: drag to pan, click to pick ====
let drag: { id: number; x: number; y: number; moved: boolean } | null = null

function down(e: PointerEvent) {
  drag = { id: e.pointerId, x: e.clientX, y: e.clientY, moved: false }
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function move(e: PointerEvent) {
  const rect = el.value!.getBoundingClientRect()
  hover.value = toBlock(e.clientX - rect.left, e.clientY - rect.top)
  if (!drag || drag.id !== e.pointerId) return
  const dx = e.clientX - drag.x
  const dy = e.clientY - drag.y
  if (Math.abs(dx) + Math.abs(dy) > 3) drag.moved = true
  if (drag.moved) {
    center.value = { x: center.value.x - dx * blocksPerPx.value, z: center.value.z - dy * blocksPerPx.value }
    drag.x = e.clientX
    drag.y = e.clientY
  }
}

function up(e: PointerEvent) {
  if (drag && !drag.moved) {
    const rect = el.value!.getBoundingClientRect()
    emit('pick', toBlock(e.clientX - rect.left, e.clientY - rect.top))
  }
  drag = null
}

function zoom(delta: number) {
  scaleIndex.value = Math.min(scales.value.length - 1, Math.max(0, scaleIndex.value + delta))
}

function wheel(e: WheelEvent) {
  e.preventDefault()
  zoom(e.deltaY > 0 ? 1 : -1)
}

function recenter() {
  center.value = props.point ? { ...props.point } : { x: props.info.spawn?.x ?? 0, z: props.info.spawn?.z ?? 0 }
}

const marker = computed(() => (props.point ? toScreen(props.point.x, props.point.z) : null))
const spawnPos = computed(() =>
  props.dimension === 'overworld' && props.info.spawn ? toScreen(props.info.spawn.x, props.info.spawn.z) : null,
)
const stamp = computed(() => (assets.stamp ? `?v=${encodeURIComponent(assets.stamp)}` : ''))
</script>

<template>
  <div class="map-frame">
    <div
      ref="el"
      class="map"
      role="application"
      aria-label="Map. Drag to move, click to pick the delivery point."
      @pointerdown="down"
      @pointermove="move"
      @pointerup="up"
      @pointerleave="hover = null"
      @wheel="wheel"
    >
      <img
        v-for="t in tiles"
        :key="t.key"
        class="tile"
        :src="t.src"
        :style="{ left: t.left + 'px', top: t.top + 'px', width: tile * PX + 'px', height: tile * PX + 'px' }"
        alt=""
        draggable="false"
      />
      <img
        v-for="v in villages"
        :key="`${v.x}_${v.z}`"
        class="deco"
        :src="`/mc/gui/map_${v.type}.png${stamp}`"
        :style="{ left: toScreen(v.x, v.z).left + 'px', top: toScreen(v.x, v.z).top + 'px' }"
        :title="`Village at ${v.x}, ${v.z}`"
        alt=""
      />
      <img
        v-if="spawnPos"
        class="deco"
        :src="`/mc/gui/map_spawn.png${stamp}`"
        :style="{ left: spawnPos.left + 'px', top: spawnPos.top + 'px' }"
        title="World spawn"
        alt=""
      />
      <img
        v-if="marker"
        class="deco target"
        :src="`/mc/gui/map_red_x.png${stamp}`"
        :style="{ left: marker.left + 'px', top: marker.top + 'px' }"
        alt=""
      />
      <span class="readout shadow">{{ hover ? `X ${hover.x}  Z ${hover.z}` : `1 pixel = ${scale} blocks` }}</span>
    </div>
    <div class="controls">
      <button type="button" class="ctl shadow" :disabled="scaleIndex === 0" aria-label="Zoom in" @click="zoom(-1)">+</button>
      <button type="button" class="ctl shadow" :disabled="scaleIndex === scales.length - 1" aria-label="Zoom out" @click="zoom(1)">
        -
      </button>
      <button type="button" class="ctl shadow wide" @click="recenter">{{ point ? 'Go to pick' : 'Go to spawn' }}</button>
    </div>
  </div>
</template>

<style scoped>
/* Framed like a map item: the game's paper border around the view. */
.map-frame {
  position: relative;
  border: calc(var(--u) * 4) solid #c5a87a;
  border-image: url('/mc/gui/map_background.png') 4 / calc(var(--u) * 4) stretch;
}

.map {
  position: relative;
  height: calc(var(--u) * 150);
  overflow: hidden;
  background: #000;
  cursor: crosshair;
  touch-action: none;
  user-select: none;
}

.tile {
  position: absolute;
  image-rendering: pixelated;
  pointer-events: none;
}

.deco {
  position: absolute;
  width: calc(var(--u) * 8);
  height: calc(var(--u) * 8);
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.deco.target {
  width: calc(var(--u) * 10);
  height: calc(var(--u) * 10);
  z-index: 2;
}

.readout {
  position: absolute;
  left: calc(var(--u) * 2);
  bottom: calc(var(--u) * 1);
  color: var(--text);
  pointer-events: none;
}

.controls {
  position: absolute;
  top: calc(var(--u) * 2);
  right: calc(var(--u) * 2);
  display: flex;
  gap: var(--u);
}

.ctl {
  min-width: calc(var(--u) * 14);
  height: calc(var(--u) * 14);
  padding: 0 calc(var(--u) * 2);
  border: var(--u) solid #000;
  background: rgba(0, 0, 0, 0.6);
  color: var(--text);
  cursor: pointer;
}

.ctl:hover:not(:disabled) {
  color: var(--yellow);
}

.ctl:disabled {
  color: var(--dark-gray);
  cursor: default;
}
</style>
