import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { api } from '@/lib/api/client'
import type { History, ProductDetail } from '@/lib/api/products'
import ProductPage from '@/routes/product/page'

vi.mock('@/lib/api/client', () => ({
  api: { get: vi.fn(), patch: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

const detail: ProductDetail = {
  id: 1,
  name: 'AirPods Pro 3',
  paused: false,
  linkCount: 1,
  bestCents: 31500,
  bestSite: 'shopee',
  typicalCents: 32900,
  pctVsTypical: -4.3,
  spark: [],
  warnings: [],
  settings: { dropPct: 15, windowDays: 30, minHistoryDays: 2 },
  overrides: { dropPct: 15, windowDays: null, minHistoryDays: null },
  links: [
    {
      id: 7,
      url: 'https://shopee.sg/product/1/2',
      site: 'shopee',
      title: 'Apple AirPods Pro 3',
      seller: 'Apple Flagship Store',
      status: 'ok',
      lastError: '',
      lastCheckedAt: '2026-10-18T01:41:00Z',
      variants: [
        {
          id: 11,
          externalId: 'm1',
          name: 'Pro 3',
          tracked: true,
          isNew: false,
          gone: false,
          inStock: true,
          priceCents: 31600,
          bestCents: 31500,
          bestIsEstimate: false,
          originalCents: 34900,
          checkedAt: '2026-10-18T01:41:00Z',
        },
      ],
    },
  ],
}

const history: History = {
  series: [{ variantId: 11, label: 'Shopee · Apple Flagship Store · Pro 3', points: [{ t: '2026-10-18T01:41:00Z', cents: 31500 }] }],
  typicalCents: 32900,
}

beforeEach(() => {
  vi.mocked(api.get).mockImplementation((path: string) =>
    Promise.resolve(path.endsWith('/history') ? history : detail),
  )
  vi.mocked(api.patch).mockResolvedValue(detail)
})

function renderPage() {
  return renderWithProviders(
    <MemoryRouter initialEntries={['/products/1']}>
      <Routes>
        <Route path="products/:id" element={<ProductPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('ProductPage', () => {
  it('shows best price, typical price and the alert threshold', async () => {
    renderPage()
    const stats = await screen.findByTestId('product-stats')

    expect(within(stats).getByText('$315.00')).toBeInTheDocument()
    expect(within(stats).getByText('Shopee')).toBeInTheDocument()
    expect(within(stats).getByText('$329.00')).toBeInTheDocument()
    expect(within(stats).getByText('15% under')).toBeInTheDocument()
  })

  it('stops tracking a variant when you untick it', async () => {
    renderPage()
    await userEvent.click(await screen.findByRole('checkbox', { name: 'Track Pro 3' }))

    expect(api.patch).toHaveBeenCalledWith('/variants/11', { tracked: false })
  })

  it('clears a per product override back to the default', async () => {
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: 'Use default drop %' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save alert settings' }))

    expect(api.patch).toHaveBeenCalledWith('/products/1', { dropPct: null, windowDays: null, minHistoryDays: null })
  })
})
