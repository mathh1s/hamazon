import { ref, watch } from 'vue'
import { readJSON, writeJSON } from '../lib/storage'

export const soundOn = ref(readJSON<boolean>('hamazon.sound', false))
watch(soundOn, (v) => writeJSON('hamazon.sound', v))

let ctx: AudioContext | null = null

function audio() {
  if (!ctx) {
    const Ctor = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctor) return null
    ctx = new Ctor()
  }
  return ctx
}

// Synthesized stand-ins for the button click and item pickup pop, so no
// game sound files are needed.
function blip(freq: number, endFreq: number, duration: number, type: OscillatorType, volume: number) {
  if (!soundOn.value) return
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

export const playClick = () => blip(900, 500, 0.06, 'square', 0.05)
export const playPop = () => blip(500, 1400, 0.09, 'sine', 0.12)
export const playLevelUp = () => {
  blip(520, 780, 0.12, 'triangle', 0.1)
  setTimeout(() => blip(780, 1040, 0.18, 'triangle', 0.1), 110)
}
