<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { ServiceSource } from '../types'
import { useCopied } from '../composables/useCopied'
import Icon from './Icon.vue'

type KindFilter = 'all' | 'path' | 'ua'

const props = defineProps<{ sources: ServiceSource[] }>()

const origin = window.location.origin
const query = ref('')
const kind = ref<KindFilter>('all')
const selectedKey = ref(props.sources[0]?.key ?? '')

const kindOptions: { id: KindFilter; label: string }[] = [
  { id: 'all', label: '全部' },
  { id: 'path', label: '路径分流' },
  { id: 'ua', label: 'UA 分流' },
]

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return props.sources.filter((s) => {
    if (kind.value === 'path' && s.ua) return false
    if (kind.value === 'ua' && !s.ua) return false
    if (!q) return true
    return [s.key, s.path, s.prefix, s.ua, s.mirror].join(' ').toLowerCase().includes(q)
  })
})

const selected = computed(() => {
  const hit = filtered.value.find((s) => s.key === selectedKey.value)
  return hit ?? filtered.value[0] ?? null
})

watch(filtered, (list) => {
  if (list.length > 0 && !list.some((s) => s.key === selectedKey.value)) {
    selectedKey.value = list[0].key
  }
})

const { copiedKey, copy } = useCopied()

function sourceUrl(s: ServiceSource): string {
  return `${origin}${s.path || s.prefix || ''}`
}
</script>

<template>
  <section id="sources" class="section" aria-labelledby="sources-title">
    <header class="section-head">
      <p class="eyebrow">Catalog</p>
      <h2 id="sources-title">镜像目录</h2>
      <p class="section-desc">当前站点可用的镜像源，支持按关键字搜索与分流方式筛选。</p>
    </header>

    <div class="card">
      <div class="toolbar">
        <div class="search-box">
          <Icon name="search" :size="15" class="search-icon" />
          <input
            v-model="query"
            type="search"
            :placeholder="'搜索镜像源，如 pypi、docker、ubuntu'"
            aria-label="搜索镜像源"
          />
        </div>
        <div class="seg" role="group" aria-label="按分流方式筛选">
          <button
            v-for="opt in kindOptions"
            :key="opt.id"
            type="button"
            class="seg-btn"
            :class="{ active: kind === opt.id }"
            :aria-pressed="kind === opt.id"
            @click="kind = opt.id"
          >
            {{ opt.label }}
          </button>
        </div>
        <span class="muted-text toolbar-count">共 {{ filtered.length }} 个</span>
      </div>

      <p v-if="filtered.length === 0" class="empty-note">没有匹配「{{ query }}」的镜像源。</p>

      <div v-else class="source-grid" role="list">
        <button
          v-for="s in filtered"
          :key="s.key"
          type="button"
          class="source-chip"
          :class="{ active: selected?.key === s.key }"
          role="listitem"
          @click="selectedKey = s.key"
        >
          <span class="source-chip-key mono">{{ s.key }}</span>
          <span class="source-chip-path mono">{{ s.path || s.prefix || '/' }}</span>
        </button>
      </div>

      <Transition name="panel" mode="out-in">
        <article v-if="selected" :key="selected.key" class="source-detail">
          <div class="source-detail-head">
            <h3 class="mono">{{ selected.key }}</h3>
            <div class="badge-row">
              <span v-if="selected.ua" class="badge">UA: {{ selected.ua }}</span>
              <span v-if="selected.priority > 0" class="badge">优先级 {{ selected.priority }}</span>
            </div>
          </div>
          <div class="detail-grid">
            <div class="detail-item">
              <span class="detail-label">Path</span>
              <code class="detail-value">{{ selected.path || '/' }}</code>
            </div>
            <div v-if="selected.prefix" class="detail-item">
              <span class="detail-label">Prefix</span>
              <code class="detail-value">{{ selected.prefix }}</code>
            </div>
            <div class="detail-item">
              <span class="detail-label">Upstream</span>
              <code class="detail-value">{{ selected.mirror }}</code>
            </div>
            <div class="detail-item detail-wide">
              <span class="detail-label">访问地址</span>
              <div class="detail-copy-row">
                <code class="detail-value">{{ sourceUrl(selected) }}</code>
                <button
                  type="button"
                  class="copy-btn copy-btn-light"
                  :class="{ copied: copiedKey === `url-${selected.key}` }"
                  @click="copy(`url-${selected.key}`, sourceUrl(selected))"
                >
                  <Icon :name="copiedKey === `url-${selected.key}` ? 'check' : 'copy'" :size="14" />
                  {{ copiedKey === `url-${selected.key}` ? '已复制' : '复制' }}
                </button>
              </div>
            </div>
          </div>
        </article>
      </Transition>
    </div>
  </section>
</template>
