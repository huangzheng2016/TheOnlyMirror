<script setup lang="ts">
import { computed, ref } from 'vue'
import { profiles, type ProfileId } from '../data/profiles'
import { highlightBashLine } from '../lib/highlight'
import { useCopied } from '../composables/useCopied'
import Icon from './Icon.vue'

const origin = window.location.origin
const selected = ref<ProfileId>('docker')

const tabEls = new Map<ProfileId, HTMLButtonElement>()
function setTabEl(id: ProfileId, el: unknown) {
  if (el instanceof HTMLButtonElement) tabEls.set(id, el)
}

const current = computed(() => profiles.find((p) => p.id === selected.value) ?? profiles[0])
const commandText = computed(() => current.value.buildCommands(origin))
const lines = computed(() => commandText.value.split('\n'))
const highlighted = computed(() => lines.value.map((line) => highlightBashLine(line)))

const { copiedKey, copy } = useCopied()
const copyKey = computed(() => `guide-${selected.value}`)

function onTablistKeydown(e: KeyboardEvent) {
  const ids = profiles.map((p) => p.id)
  const index = ids.indexOf(selected.value)
  let next = -1
  if (e.key === 'ArrowRight') next = (index + 1) % ids.length
  else if (e.key === 'ArrowLeft') next = (index - 1 + ids.length) % ids.length
  else if (e.key === 'Home') next = 0
  else if (e.key === 'End') next = ids.length - 1
  if (next < 0) return
  e.preventDefault()
  selected.value = ids[next]
  tabEls.get(ids[next])?.focus()
}
</script>

<template>
  <section id="guide" class="section" aria-labelledby="guide-title">
    <header class="section-head">
      <p class="eyebrow">Guide</p>
      <h2 id="guide-title">换源指南</h2>
      <p class="section-desc">选择你的生态，复制命令执行，即可切换到当前镜像站。</p>
    </header>

    <div class="card guide-card">
      <div class="tabs" role="tablist" aria-label="选择生态" @keydown="onTablistKeydown">
        <button
          v-for="p in profiles"
          :key="p.id"
          :ref="(el) => setTabEl(p.id, el)"
          type="button"
          class="tab"
          :class="{ active: selected === p.id }"
          role="tab"
          :id="`tab-${p.id}`"
          :aria-selected="selected === p.id"
          :aria-controls="`panel-${p.id}`"
          :tabindex="selected === p.id ? 0 : -1"
          @click="selected = p.id"
        >
          <span class="tab-label">{{ p.label }}</span>
          <span class="tab-desc">{{ p.desc }}</span>
        </button>
      </div>

      <Transition name="panel" mode="out-in">
        <div
          :key="selected"
          class="guide-panel"
          role="tabpanel"
          :id="`panel-${selected}`"
          :aria-labelledby="`tab-${selected}`"
        >
          <div class="editor">
            <div class="editor-bar">
              <span class="dot red"></span>
              <span class="dot yellow"></span>
              <span class="dot green"></span>
              <span class="editor-title">{{ current.label }} · 换源命令</span>
              <button
                type="button"
                class="copy-btn"
                :class="{ copied: copiedKey === copyKey }"
                @click="copy(copyKey, commandText)"
              >
                <Icon :name="copiedKey === copyKey ? 'check' : 'copy'" :size="14" />
                {{ copiedKey === copyKey ? '已复制' : '复制命令' }}
              </button>
            </div>
            <div class="editor-body">
              <div v-for="(html, index) in highlighted" :key="`${selected}-${index}`" class="editor-line">
                <span class="line-no">{{ index + 1 }}</span>
                <code class="hljs" v-html="html"></code>
              </div>
            </div>
          </div>
          <p class="guide-note">{{ current.note }}</p>
        </div>
      </Transition>
    </div>
  </section>
</template>
