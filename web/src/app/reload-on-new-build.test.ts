import { describe, expect, it, vi } from 'vitest'
import { reloadOnNewBuild, type ReloadTarget } from '@/app/reload-on-new-build'

function fakeWindow() {
  const events = new EventTarget()
  const store = new Map<string, string>()
  const win: ReloadTarget = {
    addEventListener: events.addEventListener.bind(events),
    sessionStorage: {
      getItem: (k) => store.get(k) ?? null,
      setItem: (k, v) => void store.set(k, v),
    },
    location: { reload: vi.fn() },
  }
  const fail = () => {
    const event = new Event('vite:preloadError', { cancelable: true })
    events.dispatchEvent(event)
    return event
  }
  return { win, fail }
}

describe('reloadOnNewBuild', () => {
  it('reloads when a file from an older build fails to load', () => {
    const { win, fail } = fakeWindow()
    reloadOnNewBuild(win, () => 1_000_000)

    expect(fail().defaultPrevented).toBe(true)
    expect(win.location.reload).toHaveBeenCalledTimes(1)
  })

  it('shows the error instead of reloading again right away', () => {
    const { win, fail } = fakeWindow()
    let now = 1_000_000
    reloadOnNewBuild(win, () => now)

    fail()
    now += 2_000
    expect(fail().defaultPrevented).toBe(false)
    expect(win.location.reload).toHaveBeenCalledTimes(1)

    now += 60_000
    fail()
    expect(win.location.reload).toHaveBeenCalledTimes(2)
  })
})
