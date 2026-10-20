function isIPv4(hostname: string): boolean {
  const parts = hostname.split('.')
  if (parts.length !== 4) return false
  return parts.every((part) => /^\d+$/.test(part) && Number(part) >= 0 && Number(part) <= 255)
}

// Maps an alias to a host reachable under the current deployment, e.g.
// alias "gh" on mirror.example.com -> gh.mirror.example.com.
// localhost keeps the alias form, bare IPv4 falls back to nip.io wildcards.
export function buildAliasDomain(alias: string, host: string): string {
  const parsed = new URL(`http://${host}`)
  const port = parsed.port ? `:${parsed.port}` : ''
  const hostname = parsed.hostname
  if (hostname === 'localhost' || hostname === '::1' || hostname === '[::1]') {
    return `${alias}.localhost${port}`
  }
  if (isIPv4(hostname)) return `${alias}.${hostname}.nip.io${port}`
  return `${alias}.${hostname}${port}`
}
