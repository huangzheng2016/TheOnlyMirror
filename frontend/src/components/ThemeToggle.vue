<script setup lang="ts">
import { useTheme } from '../composables/useTheme'
import Icon from './Icon.vue'

const { darkMode, toggleTheme } = useTheme()
</script>

<template>
  <button
    type="button"
    class="icon-btn"
    :aria-pressed="darkMode"
    :aria-label="darkMode ? '切换到日间模式' : '切换到夜间模式'"
    :title="darkMode ? '切换到日间模式' : '切换到夜间模式'"
    @click="toggleTheme"
  >
    <Icon :name="darkMode ? 'sun' : 'moon'" :size="17" />
  </button>
</template>
