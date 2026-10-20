export interface ServiceSource {
  key: string
  priority: number
  ua: string
  path: string
  prefix: string
  mirror: string
}

export interface ServiceProxy {
  scheme: string
  host: string
}

export interface ServiceAlias {
  alias: string
  upstream: string
}

export interface ServicesResponse {
  sources: ServiceSource[]
  proxy: ServiceProxy[]
  hostAliases: ServiceAlias[] | Record<string, string>
  requestHost?: string
}
