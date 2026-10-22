import { ref, watch } from 'vue'
import { readJSON, writeJSON } from '../lib/storage'
import { assets } from './assets'

export const soundOn = ref(readJSON<boolean>('hamazon.sound', false))
watch(soundOn, (v) => {
  writeJSON('hamazon.sound', v)
  if (v) preload()
})

type SoundName = 'click' | 'pop' | 'levelup' | 'orb' | 'chestopen' | 'chestclosed'

let ctx: AudioContext | null = null
const buffers = new Map<SoundName, AudioBuffer | null>()
const loading = new Map<SoundName, Promise<AudioBuffer | null>>()

function audio() {
  if (!ctx) {
    const Ctor = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctor) return null
    ctx = new Ctor()
  }
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

// The real game sounds are Ogg Vorbis, which some browsers (older Safari)
// cannot decode. Those get a synthesized stand-in instead of silence.
function load(name: SoundName): Promise<AudioBuffer | null> {
  const known = buffers.get(name)
  if (known !== undefined) return Promise.resolve(known)
  let p = loading.get(name)
  if (p) return p
  const a = audio()
  if (!a || !assets.sounds.includes(name)) return Promise.resolve(null)
  p = fetch(`/mc/sound/${name}.ogg?v=${encodeURIComponent(assets.stamp)}`)
    .then((r) => (r.ok ? r.arrayBuffer() : Promise.reject()))
    .then((data) => a.decodeAudioData(data))
    .catch(() => null)
    .then((buf) => {
      buffers.set(name, buf)
      return buf
    })
  loading.set(name, p)
  return p
}

function preload() {
  for (const n of ['click', 'pop', 'levelup', 'orb', 'chestopen', 'chestclosed'] as SoundName[]) void load(n)
}

function blip(freq: number, endFreq: number, duration: number, type: OscillatorType, volume: number) {
  const a = audio()
  if (!a) return
  const t = a.currentTime
  const osc = a.createOscillator()
  const gain = a.createGain()
  osc.type = type
  osc.frequency.setValueAtTime(freq, t)
  osc.frequency.exponentialRampToValueAtTime(endFreq, t + duration)
  gain.gain.setValueAtTime(volume, t)
  gain.gain.exponentialRampToValueAtTime(0.0001, t + duration)
  osc.connect(gain).connect(a.destination)
  osc.start(t)
  osc.stop(t + duration)
}

const fallback: Record<SoundName, () => void> = {
  click: () => blip(900, 500, 0.06, 'square', 0.05),
  pop: () => blip(500, 1400, 0.09, 'sine', 0.12),
  orb: () => blip(1200, 1800, 0.12, 'sine', 0.08),
  levelup: () => {
    blip(520, 780, 0.12, 'triangle', 0.1)
    setTimeout(() => blip(780, 1040, 0.18, 'triangle', 0.1), 110)
  },
  chestopen: () => blip(180, 120, 0.15, 'sawtooth', 0.04),
  chestclosed: () => blip(140, 90, 0.12, 'sawtooth', 0.04),
}

// Pitch matches the game: pickups vary slightly, clicks are at 1.0.
async function play(name: SoundName, volume = 0.5, pitch = 1) {
  if (!soundOn.value) return
  const buf = await load(name)
  const a = audio()
  if (!a) return
  if (!buf) {
    fallback[name]()
    return
  }
  const src = a.createBufferSource()
  src.buffer = buf
  src.playbackRate.value = pitch
  const gain = a.createGain()
  gain.gain.value = volume
  src.connect(gain).connect(a.destination)
  src.start()
}

export const playClick = () => play('click', 0.25)
export const playPop = () => play('pop', 0.3, 1.4 + Math.random() * 0.6)
export const playLevelUp = () => play('levelup', 0.5)
export const playOrb = () => play('orb', 0.35, 0.8 + Math.random() * 0.4)
export const playChestOpen = () => play('chestopen', 0.3, 0.9 + Math.random() * 0.1)
export const playChestClose = () => play('chestclosed', 0.3, 0.9 + Math.random() * 0.1)

// Sounds may be on from a previous visit, so warm the cache on first input.
if (soundOn.value) {
  window.addEventListener('pointerdown', preload, { once: true })
}
