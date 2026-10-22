import { reactive } from 'vue'

export interface ManifestItem {
  id: string
  name: string
  kind: 'flat' | 'block'
}

export const assets = reactive<{
  loaded: boolean
  missing: boolean
  version: string
  stamp: string
  sounds: string[]
  items: Map<string, ManifestItem>
}>({
  loaded: false,
  missing: false,
  version: '',
  stamp: '',
  sounds: [],
  items: new Map(),
})

let polling = false

async function fetchManifest() {
  const res = await fetch('/mc/manifest.json', { cache: 'no-store' })
  if (!res.ok) throw new Error()
  const m = (await res.json()) as { version: string; generatedAt: string; sounds?: string[]; items: ManifestItem[] }
  assets.sounds = m.sounds ?? []
  assets.version = m.version
  assets.stamp = m.generatedAt
  assets.items = new Map(m.items.map((i) => [i.id, i]))
  assets.missing = false
}

// On a fresh install the server is still downloading the game assets. Keep
// checking, then reload once so the sprites in the stylesheet and the font
// are fetched too.
function pollUntilReady() {
  if (polling) return
  polling = true
  const timer = setInterval(async () => {
    try {
      await fetchManifest()
      clearInterval(timer)
      location.reload()
    } catch {
      // Still downloading.
    }
  }, 10_000)
}

export async function loadAssets() {
  if (assets.loaded) return
  try {
    await fetchManifest()
  } catch {
    assets.missing = true
    pollUntilReady()
  } finally {
    assets.loaded = true
  }
}
