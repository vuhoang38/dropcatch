/**
 * Render routes in a real browser and save what they look like.
 *
 * Usage:
 *   bun run screenshot            # captures /
 *   bun run screenshot /users     # captures one route
 *   bun run screenshot / /users /settings /login
 *   bun run screenshot --dark /users   # captures in dark mode
 *
 * Images land in .screenshots/. Open them and actually look at them. Code review
 * does not catch a broken grid or a heading that collides with a button.
 *
 * The browser console is captured too. Any console error or uncaught exception
 * exits non-zero, which turns a silent runtime crash into a visible failure.
 */
import { chromium, type ConsoleMessage } from 'playwright'
import { mkdir, rm } from 'node:fs/promises'

const HOST = 'http://localhost:5173'
const OUT_DIR = '.screenshots'

const VIEWPORTS = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'mobile', width: 390, height: 844 },
]

/** Messages that are noise rather than a defect. */
const IGNORED = [/Download the React DevTools/i, /\[vite\] (connected|connecting)/i]

const argv = process.argv.slice(2)
const dark = argv.includes('--dark')
const routes = argv.filter((arg) => !arg.startsWith('--'))
if (routes.length === 0) routes.push('/')

function slug(route: string) {
  const trimmed = route.replace(/^\/+|\/+$/g, '')
  return trimmed === '' ? 'home' : trimmed.replaceAll('/', '-')
}

async function serverIsUp() {
  try {
    const response = await fetch(HOST, { signal: AbortSignal.timeout(1000) })
    return response.ok
  } catch {
    return false
  }
}

async function waitForServer(timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (await serverIsUp()) return true
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  return false
}

async function main() {
  let devServer: ReturnType<typeof Bun.spawn> | undefined

  if (!(await serverIsUp())) {
    console.log('Dev server not running. Starting it...')
    devServer = Bun.spawn(['bun', 'run', 'dev'], { stdout: 'ignore', stderr: 'ignore' })
    if (!(await waitForServer())) {
      devServer.kill()
      throw new Error(`Dev server did not come up at ${HOST} within 30s.`)
    }
  }

  await rm(OUT_DIR, { recursive: true, force: true })
  await mkdir(OUT_DIR, { recursive: true })

  const browser = await chromium.launch()
  const problems: string[] = []

  try {
    for (const route of routes) {
      for (const viewport of VIEWPORTS) {
        const context = await browser.newContext({
          viewport: { width: viewport.width, height: viewport.height },
          deviceScaleFactor: 2,
          // The app follows the system preference by default, so this is all it
          // takes to capture the other theme.
          colorScheme: dark ? 'dark' : 'light',
        })
        const page = await context.newPage()

        const noteProblem = (text: string) => {
          if (IGNORED.some((pattern) => pattern.test(text))) return
          problems.push(`${route} [${viewport.name}] ${text}`)
        }

        page.on('console', (message: ConsoleMessage) => {
          if (message.type() === 'error') noteProblem(`console.error: ${message.text()}`)
        })
        page.on('pageerror', (error: Error) => noteProblem(`uncaught: ${error.message}`))

        const response = await page.goto(`${HOST}${route}`, { waitUntil: 'networkidle' })
        if (response && response.status() >= 400) {
          noteProblem(`HTTP ${response.status()}`)
        }

        const file = `${OUT_DIR}/${slug(route)}-${viewport.name}${dark ? '-dark' : ''}.png`
        await page.screenshot({ path: file, fullPage: true })
        console.log(`Captured ${file}`)

        await context.close()
      }
    }
  } finally {
    await browser.close()
    devServer?.kill()
  }

  if (problems.length > 0) {
    console.error(`\n${problems.length} console problem(s):`)
    for (const problem of problems) console.error(`  - ${problem}`)
    process.exit(1)
  }

  console.log('\nNo console errors. Now open the images and look at them.')
}

await main()
