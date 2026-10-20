<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useServices } from './composables/useServices'
import { normalizeAliases } from './lib/normalize'
import SiteHeader from './components/SiteHeader.vue'
import GuideSection from './components/GuideSection.vue'
import SourcesSection from './components/SourcesSection.vue'
import AliasesSection from './components/AliasesSection.vue'
import ProxySection from './components/ProxySection.vue'
import LoadError from './components/LoadError.vue'
import PageSkeleton from './components/PageSkeleton.vue'

const { loading, error, data, load } = useServices()

const aliases = computed(() => (data.value ? normalizeAliases(data.value) : []))
const stats = computed(() => ({
  sources: data.value?.sources.length ?? 0,
  aliases: aliases.value.length,
  proxy: data.value?.proxy.length ?? 0,
}))
const status = computed(() => (error.value ? 'error' : loading.value ? 'loading' : 'ok'))

onMounted(() => {
  void load()
})
</script>

<template>
  <a class="skip-link" href="#main">跳到主内容</a>
  <SiteHeader :status="status" :ready="!!data" :request-host="data?.requestHost" :stats="stats" />
  <main id="main" class="container main">
    <PageSkeleton v-if="loading && !data" />
    <LoadError v-else-if="error && !data" :message="error" @retry="load" />
    <template v-else-if="data">
      <GuideSection />
      <SourcesSection :sources="data.sources" />
      <AliasesSection :aliases="aliases" />
      <ProxySection :proxy="data.proxy" />
    </template>
  </main>
  <footer class="footer">
    <div class="container footer-inner">
      <span>TheOnlyMirror · 一站式镜像聚合加速</span>
      <span class="muted-text">页面数据由 /api/services 实时驱动</span>
    </div>
  </footer>
</template>
