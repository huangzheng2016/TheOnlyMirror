<script setup lang="ts">
import { computed } from 'vue'
import type { ServiceAlias } from '../types'
import { buildAliasDomain } from '../lib/hosts'
import { useCopied } from '../composables/useCopied'
import Icon from './Icon.vue'

const props = defineProps<{ aliases: ServiceAlias[] }>()

const host = window.location.host
const items = computed(() =>
  props.aliases.map((a) => ({ ...a, accessHost: buildAliasDomain(a.alias, host) })),
)

const { copiedKey, copy } = useCopied()
</script>

<template>
  <section id="aliases" class="section" aria-labelledby="aliases-title">
    <header class="section-head">
      <p class="eyebrow">Aliases</p>
      <h2 id="aliases-title">快捷访问</h2>
      <p class="section-desc">
        将 <code class="inline-code">{{ '{alias}' }}.{{ host }}</code> 泛解析到本站后，即可按别名访问对应上游域名。
      </p>
    </header>

    <div v-if="items.length === 0" class="card"><p class="empty-note">当前没有可用 alias。</p></div>
    <div v-else class="alias-grid">
      <article v-for="item in items" :key="item.alias" class="card alias-card">
        <div class="alias-head">
          <span class="alias-name mono">{{ item.alias }}</span>
          <span class="alias-arrow" aria-hidden="true">→</span>
          <span class="alias-upstream mono">{{ item.upstream }}</span>
        </div>
        <div class="alias-foot">
          <code class="alias-host mono">{{ item.accessHost }}</code>
          <button
            type="button"
            class="copy-btn copy-btn-light"
            :class="{ copied: copiedKey === `alias-${item.alias}` }"
            :aria-label="`复制 ${item.accessHost}`"
            @click="copy(`alias-${item.alias}`, item.accessHost)"
          >
            <Icon :name="copiedKey === `alias-${item.alias}` ? 'check' : 'copy'" :size="14" />
            {{ copiedKey === `alias-${item.alias}` ? '已复制' : '复制' }}
          </button>
        </div>
      </article>
    </div>
  </section>
</template>
