<script setup lang="ts">
import Icon from './Icon.vue'

defineProps<{ message: string }>()
const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="card error-card" role="alert">
    <p class="error-title">镜像数据加载失败</p>
    <p class="error-message mono">{{ message }}</p>
    <p class="error-hint">请检查网络连接或服务状态后重试。</p>
    <button type="button" class="btn btn-primary" @click="emit('retry')">
      <Icon name="refresh" :size="15" />
      重新加载
    </button>
  </div>
</template>
