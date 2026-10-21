// crypto.randomUUID only exists on https or localhost, but the shop may be
// opened over plain http on a LAN, so build the id from random bytes.
export function randomId(bytes = 16) {
  const buf = new Uint8Array(bytes)
  crypto.getRandomValues(buf)
  return Array.from(buf, (b) => b.toString(16).padStart(2, '0')).join('')
}
