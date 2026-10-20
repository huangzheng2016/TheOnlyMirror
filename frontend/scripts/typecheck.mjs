// vue-tsc 3.3.x type-checks .vue files by loading the JavaScript tsc module
// and instrumenting its source. typescript@7 only ships the native tsgo
// binary and its exports map hides ./lib/tsc, so the default resolution
// fails. vue-tsc's public run(tscPath) API accepts an explicit tsc path, and
// @typescript/typescript6 is the supported bridge to the full JavaScript tsc
// (provided via @typescript/old); its package name is detected by vue-tsc.
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const { run } = require('vue-tsc')

run(require.resolve('@typescript/typescript6/lib/tsc'))
