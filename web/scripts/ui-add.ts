/**
 * Wrapper around `shadcn add` that keeps generated components consistent with
 * this template's conventions.
 *
 * The shadcn registry currently imports `cn` from the standalone `cn` package.
 * This template standardises on `@/lib/utils`, so every freshly generated file
 * is rewritten after the CLI runs. Without this, each `ui:add` would reintroduce
 * a second source of truth for `cn`.
 */
import { $ } from 'bun'
import { readdir, readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'

const UI_DIR = 'src/components/ui'

const args = process.argv.slice(2)
if (args.length === 0) {
  console.error('Usage: bun run ui:add <component> [...components]')
  process.exit(1)
}

await $`bunx --bun shadcn@latest add ${args} --yes`.nothrow()

let patched = 0
for (const file of await readdir(UI_DIR)) {
  if (!file.endsWith('.tsx')) continue
  const path = join(UI_DIR, file)
  const before = await readFile(path, 'utf8')
  const after = before.replaceAll('from "cn"', 'from "@/lib/utils"')
  if (after !== before) {
    await writeFile(path, after)
    patched++
  }
}

console.log(patched > 0 ? `Normalised cn imports in ${patched} file(s).` : 'Imports already consistent.')
