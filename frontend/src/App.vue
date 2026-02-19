<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'

interface ServiceSource {
  key: string
  priority: number
  ua: string
  path: string
  prefix: string
  mirror: string
}

interface ServiceProxy {
  scheme: string
  host: string
}

interface ServiceAlias {
  alias: string
  upstream: string
}

interface ServicesResponse {
  sources: ServiceSource[]
  proxy: ServiceProxy[]
  hostAliases: ServiceAlias[] | Record<string, string>
}

type Profile = 'docker' | 'linux' | 'python' | 'node' | 'go' | 'github'

hljs.registerLanguage('bash', bash)

const themeKey = 'mirror-theme-dark'
const darkMode = ref(false)
const loading = ref(true)
const fetchError = ref('')
const services = ref<ServicesResponse | null>(null)
const selectedProfile = ref<Profile>('docker')
const selectedSourceKey = ref('')
const sourceQuery = ref('')

const runtimeOrigin = computed(() => window.location.origin)
const runtimeHost = computed(() => window.location.host)

const profileMeta: Record<Profile, { label: string; desc: string }> = {
  docker: { label: 'Docker', desc: 'registry mirror' },
  linux: { label: 'Linux', desc: 'apt/apk 源替换' },
  python: { label: 'Python', desc: 'pip / pypi' },
  node: { label: 'Node', desc: 'npm registry' },
  go: { label: 'Golang', desc: 'goproxy' },
  github: { label: 'GitHub', desc: '文件下载加速' },
}
const profileKeys = computed(() => Object.keys(profileMeta) as Profile[])

const sourceList = computed(() => services.value?.sources ?? [])
const filteredSources = computed(() => {
  const q = sourceQuery.value.trim().toLowerCase()
  if (!q) return sourceList.value
  return sourceList.value.filter((item) => {
    const text = [item.key, item.path, item.ua, item.mirror].join(' ').toLowerCase()
    return text.includes(q)
  })
})

const selectedSource = computed(() => {
  if (!sourceList.value.length) return null
  const hit = sourceList.value.find((item) => item.key === selectedSourceKey.value)
  return hit ?? sourceList.value[0]
})

function isIPv4(hostname: string): boolean {
  const parts = hostname.split('.')
  if (parts.length !== 4) return false
  return parts.every((part) => /^\d+$/.test(part) && Number(part) >= 0 && Number(part) <= 255)
}

function buildAliasDomain(alias: string, host: string): string {
  const parsed = new URL(`http://${host}`)
  const hostname = parsed.hostname
  const port = parsed.port ? `:${parsed.port}` : ''
  if (hostname === 'localhost') return `${alias}.localhost${port}`
  if (isIPv4(hostname)) return `${alias}.${hostname}.nip.io${port}`
  return `${alias}.${hostname}${port}`
}

const aliasExamples = computed(() => {
  if (!services.value) return []
  const aliases = Array.isArray(services.value.hostAliases)
    ? services.value.hostAliases
    : Object.entries(services.value.hostAliases).map(([alias, upstream]) => ({ alias, upstream }))
  return aliases.map((item) => ({
    ...item,
    accessHost: buildAliasDomain(item.alias, runtimeHost.value),
  }))
})

const proxyGroups = computed(() => {
  const list = services.value?.proxy ?? []
  const groups = new Map<string, string[]>()
  for (const item of list) {
    const scheme = item.scheme || 'https'
    const hosts = groups.get(scheme) ?? []
    hosts.push(item.host)
    groups.set(scheme, hosts)
  }
  return Array.from(groups.entries())
    .map(([scheme, hosts]) => ({
      scheme,
      hosts: [...new Set(hosts)].sort((a, b) => a.localeCompare(b)),
    }))
    .sort((a, b) => a.scheme.localeCompare(b.scheme))
})

const profileCommand = computed(() => {
  const base = runtimeOrigin.value
  switch (selectedProfile.value) {
    case 'linux':
      return `# Ubuntu / Debian
sed -i "s|http://archive.ubuntu.com/ubuntu|${base}/ubuntu|g" /etc/apt/sources.list
sed -i "s|https://deb.debian.org/debian|${base}/debian|g" /etc/apt/sources.list

# Kali / Alpine
sed -i "s|https://http.kali.org/kali|${base}/kali|g" /etc/apt/sources.list
sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${base}/alpine|g" /etc/apk/repositories`
    case 'python':
      return `pip config set global.index-url ${base}
pip install -i ${base} -r requirements.txt`
    case 'node':
      return `npm config set registry ${base}`
    case 'go':
      return `go env -w GO111MODULE=on
go env -w GOPROXY=${base}`
    case 'github':
      return `curl -O ${base}/github.com/owner/repo/archive/main.zip
wget ${base}/raw.githubusercontent.com/owner/repo/main/README.md`
    default:
      return `tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": ["${base}"]
}
EOF`
  }
})

const commandLines = computed(() => profileCommand.value.split('\n'))
const highlightedLines = computed(() =>
  commandLines.value.map((line) => {
    if (line === '') return '&nbsp;'
    return hljs.highlight(line, { language: 'bash' }).value
  }),
)

function applyTheme() {
  document.documentElement.setAttribute('data-theme', darkMode.value ? 'dark' : 'light')
}

function toggleTheme() {
  darkMode.value = !darkMode.value
  localStorage.setItem(themeKey, darkMode.value ? '1' : '0')
  applyTheme()
}

async function loadServices() {
  loading.value = true
  fetchError.value = ''
  try {
    const resp = await fetch('/api/services')
    if (!resp.ok) throw new Error('request failed: ' + resp.status)
    const payload = (await resp.json()) as ServicesResponse
    services.value = payload
    const firstSource = payload.sources[0]
    if (firstSource) selectedSourceKey.value = firstSource.key
  } catch (error) {
    fetchError.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const saved = localStorage.getItem(themeKey)
  darkMode.value = saved === null ? window.matchMedia('(prefers-color-scheme: dark)').matches : saved === '1'
  applyTheme()
  void loadServices()
})
</script>

<template>
  <main class="home">
    <div class="bg-blur bg1"></div>
    <div class="bg-blur bg2"></div>

    <header class="glass hero">
      <div class="hero-main">
        <p class="badge">THE ONLY MIRROR</p>
        <h1>聚合镜像加速站</h1>
        <p class="desc">
          All in one 一站式镜像聚合加速服务，提供统一入口，基于 UA / Path / Host / Header 等请求特征智能分流，自动路由到最匹配的源站。
        </p>
      </div>
      <button class="theme-toggle" type="button" :aria-pressed="darkMode" @click="toggleTheme">
        <span class="track" :class="{ on: darkMode }">
          <span class="thumb"></span>
        </span>
        <span>{{ darkMode ? '夜间模式' : '日间模式' }}</span>
      </button>
    </header>

    <section class="glass panel">
      <h2>换源示例</h2>
      <div class="profile-row">
        <button
          v-for="key in profileKeys"
          :key="key"
          class="profile-chip"
          :class="{ active: selectedProfile === key }"
          @click="selectedProfile = key"
        >
          <strong>{{ profileMeta[key].label }}</strong>
          <small>{{ profileMeta[key].desc }}</small>
        </button>
      </div>

      <Transition name="fade-slide" mode="out-in">
        <article :key="selectedProfile" class="command-box">
          <div class="editor-topbar">
            <span class="dot red"></span>
            <span class="dot yellow"></span>
            <span class="dot green"></span>
          </div>
          <div class="editor-body">
            <div v-for="(_, index) in commandLines" :key="'line-' + selectedProfile + '-' + index" class="editor-line">
              <span class="line-no">{{ index + 1 }}</span>
              <code class="hljs line-code" v-html="highlightedLines[index]"></code>
            </div>
          </div>
        </article>
      </Transition>
    </section>

    <section class="glass panel">
      <h2>镜像源</h2>
      <p v-if="loading">正在获取可用源...</p>
      <p v-else-if="fetchError" class="error">加载失败：{{ fetchError }}</p>
      <template v-else>
        <div class="source-search">
          <label for="source-search-input">搜索镜像源</label>
          <input
            id="source-search-input"
            v-model.trim="sourceQuery"
            type="text"
            placeholder="输入检索关键字"
          />
        </div>
        <div class="source-grid">
          <button
            v-for="item in filteredSources"
            :key="item.key"
            class="source-item"
            :class="{ active: selectedSourceKey === item.key }"
            @click="selectedSourceKey = item.key"
          >
            <span class="source-item-key">{{ item.key }}</span>
            <span class="source-item-path">{{ item.path || '/' }}</span>
          </button>
        </div>
        <p v-if="filteredSources.length === 0" class="empty-note">没有匹配到镜像源。</p>
        <Transition name="fade-slide" mode="out-in">
          <article v-if="selectedSource" :key="selectedSource.key" class="source-card">
            <h3>{{ selectedSource.key }}</h3>
            <div class="source-detail-grid">
              <p><strong>Path</strong><code>{{ selectedSource.path || '/' }}</code></p>
              <p><strong>UA</strong><code>{{ selectedSource.ua || '-' }}</code></p>
              <p><strong>Upstream</strong><code>{{ selectedSource.mirror }}</code></p>
              <p><strong>Mirror</strong><code>{{ runtimeOrigin }}{{ selectedSource.path || '/' }}</code></p>
            </div>
          </article>
        </Transition>
      </template>
    </section>

    <section class="glass panel">
      <h2>快捷访问</h2>
      <div v-if="aliasExamples.length === 0" class="empty-note">当前没有可用 alias。</div>
      <div v-else class="alias-grid">
        <article v-for="item in aliasExamples" :key="item.alias" class="alias-card">
          <p class="alias-domain"><code>{{ item.accessHost }}</code></p>
          <p class="alias-upstream">上游：<code>{{ item.upstream }}</code></p>
        </article>
      </div>
    </section>

    <section class="glass panel">
      <h2>Proxy 域名</h2>
      <div v-if="loading" class="empty-note">正在加载 Proxy 列表...</div>
      <div v-else-if="fetchError" class="empty-note">加载失败，暂时无法展示。</div>
      <div v-else-if="proxyGroups.length === 0" class="empty-note">当前未配置 Proxy 域名。</div>
      <div v-else class="proxy-groups">
        <article v-for="group in proxyGroups" :key="group.scheme" class="proxy-group-card">
          <div class="proxy-group-head">
            <span class="proxy-scheme">{{ group.scheme.toUpperCase() }}</span>
            <span class="proxy-count">{{ group.hosts.length }} 个域名</span>
          </div>
          <div class="proxy-host-list">
            <span v-for="host in group.hosts" :key="host" class="proxy-host-chip">{{ host }}</span>
          </div>
        </article>
      </div>
    </section>
  </main>
</template>
