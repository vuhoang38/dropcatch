const KEY = 'dropcatch:reloaded-at'

/** The parts of window this needs, so a test can hand in a fake. */
export type ReloadTarget = {
  addEventListener: (type: string, listener: (event: Event) => void) => void
  sessionStorage: Pick<Storage, 'getItem' | 'setItem'>
  location: { reload: () => void }
}

/**
 * Each build gives its files new names. A tab left open across an update asks
 * for the old names, and opening a page fails. Reload to pick up the new build,
 * once: if it still fails right after, show the error rather than loop.
 */
export function reloadOnNewBuild(win: ReloadTarget = window, now: () => number = Date.now) {
  win.addEventListener('vite:preloadError', (event) => {
    let last = 0
    try {
      last = Number(win.sessionStorage.getItem(KEY)) || 0
    } catch {
      // storage blocked; setItem below fails too and stops us
    }
    if (now() - last < 10_000) return
    try {
      win.sessionStorage.setItem(KEY, String(now()))
    } catch {
      return // no way to remember the reload, so don't risk a loop
    }
    event.preventDefault()
    win.location.reload()
  })
}
