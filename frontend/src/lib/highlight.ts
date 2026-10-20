import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'

hljs.registerLanguage('bash', bash)

export function highlightBashLine(line: string): string {
  if (line === '') return '&nbsp;'
  return hljs.highlight(line, { language: 'bash', ignoreIllegals: true }).value
}
