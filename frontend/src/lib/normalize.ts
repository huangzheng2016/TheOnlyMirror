import type { ServiceAlias, ServicesResponse } from '../types'

export function normalizeAliases(payload: ServicesResponse): ServiceAlias[] {
  const raw = payload.hostAliases
  if (Array.isArray(raw)) return raw
  if (raw && typeof raw === 'object') {
    return Object.entries(raw).map(([alias, upstream]) => ({ alias, upstream }))
  }
  return []
}
