<script setup lang="ts">
import ThemeToggle from './ThemeToggle.vue'

export interface HeaderStats {
  sources: number
  aliases: number
  proxy: number
}

const props = defineProps<{
  status: 'loading' | 'ok' | 'error'
  ready: boolean
  requestHost?: string
  stats: HeaderStats
}>()

const origin = window.location.origin

const statusText = { loading: '连接中', ok: '服务正常', error: '连接失败' } as const
</script>

<template>
  <header class="site-header">
    <div class="topbar">
      <div class="container topbar-inner">
        <a class="brand" href="#top" aria-label="TheOnlyMirror 首页">
          <img class="brand-mark" src="/vite.svg" alt="" width="26" height="26" />
          <span class="brand-name">TheOnlyMirror</span>
        </a>
        <nav class="nav" aria-label="页面导航">
          <a href="#guide">换源指南</a>
          <a href="#sources">镜像目录</a>
          <a href="#aliases">快捷访问</a>
          <a href="#proxy">Proxy 域名</a>
        </nav>
        <div class="topbar-right">
          <span class="status-pill" :class="`is-${props.status}`" role="status">
            <span class="status-dot"></span>
            {{ statusText[props.status] }}
          </span>
          <ThemeToggle />
        </div>
      </div>
    </div>

    <div class="container hero" id="top">
      <div class="hero-main">
        <p class="hero-badge">The Only Mirror</p>
        <h1>一个入口，<span class="accent-text">加速所有镜像</span></h1>
        <p class="hero-desc">
          All in one 一站式镜像聚合加速服务。基于 UA / Path / Host 等请求特征智能分流，
          自动路由到最匹配的源站，为 Docker、Linux、Python、Node、Go、GitHub 等生态提供统一加速入口。
        </p>
        <div v-if="props.ready" class="hero-meta">
          <span v-if="props.requestHost" class="meta-chip">
            当前入口 <strong class="mono">{{ props.requestHost }}</strong>
          </span>
          <span class="meta-chip"><strong class="mono">{{ props.stats.sources }}</strong> 个镜像源</span>
          <span class="meta-chip"><strong class="mono">{{ props.stats.aliases }}</strong> 个 alias</span>
          <span class="meta-chip"><strong class="mono">{{ props.stats.proxy }}</strong> 个 proxy 域名</span>
        </div>
        <div class="hero-actions">
          <a class="btn btn-primary" href="#guide">查看换源指南</a>
          <a class="btn btn-ghost" href="#sources">浏览镜像目录</a>
        </div>
      </div>

      <div class="editor hero-term" aria-hidden="true">
        <div class="editor-bar">
          <span class="dot red"></span>
          <span class="dot yellow"></span>
          <span class="dot green"></span>
          <span class="editor-title">quick start</span>
        </div>
        <div class="editor-body">
          <div class="editor-line">
            <span class="line-no">1</span>
            <code><span class="t-dim">$</span> <span class="t-cmd">cat</span> /etc/docker/daemon.json</code>
          </div>
          <div class="editor-line">
            <span class="line-no">2</span>
            <code><span class="t-punct">{</span></code>
          </div>
          <div class="editor-line">
            <span class="line-no">3</span>
            <code>
              <span class="t-punct">"</span><span class="t-key">registry-mirrors</span><span class="t-punct">": [</span><span class="t-punct">"</span><span class="t-str">{{ origin }}</span><span class="t-punct">"]</span>
            </code>
          </div>
          <div class="editor-line">
            <span class="line-no">4</span>
            <code><span class="t-punct">}</span></code>
          </div>
          <div class="editor-line">
            <span class="line-no">5</span>
            <code><span class="t-dim">$</span> <span class="t-cmd">docker</span> pull nginx<span class="t-cursor"></span></code>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>
