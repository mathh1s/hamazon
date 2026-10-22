import { reactive } from 'vue'
import { api, type Category, type Currency } from '../api'
import { loadSession } from './session'

export const admin = reactive({
  loaded: false,
  currencies: [] as Currency[],
  categories: [] as Category[],
  webhookConfigured: true,
  failedNotifications: 0,
  assets: { state: 'ready', version: '', error: '' } as AssetStatus,
})

export interface AssetStatus {
  state: 'ready' | 'downloading' | 'failed' | 'disabled'
  version: string
  error?: string
}

export async function loadOverview() {
  const o = await api<{
    currencies: Currency[]
    categories: Category[]
    webhookConfigured: boolean
    failedNotifications: number
    assets: AssetStatus
  }>(
    'GET',
    '/api/admin/overview',
  )
  Object.assign(admin, o, { loaded: true })
}

export async function adminLogin(name: string, password: string) {
  await api('POST', '/api/admin/login', { name, password })
  await loadSession()
}

export async function adminLogout() {
  await api('POST', '/api/admin/logout')
  await loadSession()
}
