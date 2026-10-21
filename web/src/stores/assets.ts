import { reactive } from 'vue'

export interface ManifestItem {
  id: string
  name: string
  kind: 'flat' | 'block'
}

export const assets = reactive<{ loaded: boolean; missing: boolean; version: string; items: Map<string, ManifestItem> }>({
  loaded: false,
  missing: false,
  version: '',
  items: new Map(),
})

export async function loadAssets() {
  if (assets.loaded) return
  try {
    const res = await fetch('/mc/manifest.json')
    if (!res.ok) throw new Error()
    const m = (await res.json()) as { version: string; items: ManifestItem[] }
    assets.version = m.version
    assets.items = new Map(m.items.map((i) => [i.id, i]))
  } catch {
    assets.missing = true
  } finally {
    assets.loaded = true
  }
}
