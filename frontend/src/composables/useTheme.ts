import { ref } from 'vue'

const themeKey = 'mirror-theme-dark'
const darkMode = ref(false)
let initialized = false

function applyTheme() {
  document.documentElement.setAttribute('data-theme', darkMode.value ? 'dark' : 'light')
}

export function useTheme() {
  if (!initialized) {
    initialized = true
    // index.html already applied the persisted/system theme before first paint
    darkMode.value = document.documentElement.getAttribute('data-theme') === 'dark'
  }

  function toggleTheme() {
    darkMode.value = !darkMode.value
    try {
      localStorage.setItem(themeKey, darkMode.value ? '1' : '0')
    } catch {
      // storage unavailable (private mode etc.); theme just won't persist
    }
    applyTheme()
  }

  return { darkMode, toggleTheme }
}
