import { ref } from 'vue'
import type { ServicesResponse } from '../types'

export function useServices() {
  const loading = ref(true)
  const error = ref('')
  const data = ref<ServicesResponse | null>(null)

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const resp = await fetch('/api/services', { headers: { Accept: 'application/json' } })
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      data.value = (await resp.json()) as ServicesResponse
    } catch (e) {
      data.value = null
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }

  return { loading, error, data, load }
}
