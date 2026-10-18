import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

/**
 * jsdom does not implement the browser APIs Radix primitives rely on. Without
 * these, any test that renders a Select, Dialog, Tooltip or Popover throws.
 */
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

function definePolyfill(target: object, property: string, value: unknown) {
  if (property in target) return
  Object.defineProperty(target, property, { value, writable: true, configurable: true })
}

definePolyfill(globalThis, 'ResizeObserver', ResizeObserverStub)

if (typeof window !== 'undefined') {
  definePolyfill(window, 'matchMedia', (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }))

  definePolyfill(Element.prototype, 'scrollIntoView', () => {})
  definePolyfill(Element.prototype, 'hasPointerCapture', () => false)
  definePolyfill(Element.prototype, 'setPointerCapture', () => {})
  definePolyfill(Element.prototype, 'releasePointerCapture', () => {})
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})
