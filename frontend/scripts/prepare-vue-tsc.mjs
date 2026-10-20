// vue-tsc 3.3.x type-checks .vue files by loading the JavaScript tsc module
// (typescript/lib/tsc) and instrumenting its source. typescript@7 only ships
// the native tsgo binary and its exports map hides ./lib/tsc, so resolution
// fails. @typescript/typescript6 is the compatibility bridge: it re-exports
// the full JavaScript tsc as @typescript/old, and vue-tsc detects it by
// package name. Linking it next to vue-tsc makes Node resolve that copy
// first, keeping typescript@7 as the project's real TypeScript elsewhere.
import { existsSync, lstatSync, mkdirSync, readlinkSync, rmSync, symlinkSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const projectRoot = fileURLToPath(new URL('..', import.meta.url))
const require = createRequire(import.meta.url)

const bridgeDir = dirname(require.resolve('@typescript/typescript6/package.json'))
const linkPath = join(projectRoot, 'node_modules', 'vue-tsc', 'node_modules', 'typescript')
const linkTarget = relative(dirname(linkPath), bridgeDir)

if (existsSync(linkPath)) {
  const stats = lstatSync(linkPath)
  if (stats.isSymbolicLink() && readlinkSync(linkPath) === linkTarget) {
    process.exit(0)
  }
  rmSync(linkPath, { recursive: true, force: true })
}

mkdirSync(dirname(linkPath), { recursive: true })
symlinkSync(linkTarget, linkPath, 'dir')
console.log('vue-tsc: linked @typescript/typescript6 compiler bridge (typescript@7 compat)')
