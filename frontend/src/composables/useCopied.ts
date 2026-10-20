import { ref } from 'vue'
import { copyText } from '../lib/clipboard'

// Tracks which control most recently copied, so the UI can show
// a transient "copied" state keyed by an arbitrary string.
export function useCopied(resetMs = 1600) {
  const copiedKey = ref('')
  let timer: ReturnType<typeof setTimeout> | undefined

  async function copy(key: string, text: string): Promise<boolean> {
    const ok = await copyText(text)
    if (!ok) return false
    copiedKey.value = key
    clearTimeout(timer)
    timer = setTimeout(() => {
      copiedKey.value = ''
    }, resetMs)
    return true
  }

  return { copiedKey, copy }
}
