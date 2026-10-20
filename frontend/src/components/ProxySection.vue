<script setup lang="ts">
import { computed } from 'vue'
import type { ServiceProxy } from '../types'
import { useCopied } from '../composables/useCopied'

const props = defineProps<{ proxy: ServiceProxy[] }>()

const groups = computed(() => {
  const map = new Map<string, Set<string>>()
  for (const item of props.proxy) {
    const scheme = item.scheme || 'https'
    if (!map.has(scheme)) map.set(scheme, new Set())
    map.get(scheme)!.add(item.host)
  }
  return Array.from(map.entries())
    .map(([scheme, hosts]) => ({ scheme, hosts: [...hosts].sort((a, b) => a.localeCompare(b)) }))
    .sort((a, b) => a.scheme.localeCompare(b.scheme))
})

const { copiedKey, copy } = useCopied()
</script>

<template>
  <section id="proxy" class="section" aria-labelledby="proxy-title">
    <header class="section-head">
      <p class="eyebrow">Proxy</p>
      <h2 id="proxy-title">Proxy 域名</h2>
      <p class="section-desc">以下域名由本站反向代理，点击即可复制完整地址。</p>
    </header>

    <div v-if="groups.length === 0" class="card"><p class="empty-note">当前未配置 Proxy 域名。</p></div>
    <div v-else class="card">
      <div v-for="group in groups" :key="group.scheme" class="proxy-group">
        <div class="proxy-group-head">
          <span class="badge mono">{{ group.scheme }}</span>
          <span class="muted-text">{{ group.hosts.length }} 个域名</span>
        </div>
        <div class="proxy-hosts">
          <button
            v-for="host in group.hosts"
            :key="host"
            type="button"
            class="proxy-chip mono"
            :class="{ copied: copiedKey === `proxy-${group.scheme}-${host}` }"
            :aria-label="`复制 ${group.scheme}://${host}`"
            @click="copy(`proxy-${group.scheme}-${host}`, `${group.scheme}://${host}`)"
          >
            {{ copiedKey === `proxy-${group.scheme}-${host}` ? '已复制' : host }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
