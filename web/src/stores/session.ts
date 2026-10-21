import { reactive } from 'vue'
import { api, setCsrfToken, type Session } from '../api'

export const session = reactive<{ loaded: boolean } & Session>({
  loaded: false,
  access: false,
  floodgatePrefix: '.',
  admin: null,
})

export async function loadSession() {
  const s = await api<Session>('GET', '/api/session')
  Object.assign(session, s, { loaded: true })
  setCsrfToken(s.admin?.csrfToken ?? '')
}

export async function unlock(password: string) {
  await api('POST', '/api/access', { password })
  await loadSession()
}
