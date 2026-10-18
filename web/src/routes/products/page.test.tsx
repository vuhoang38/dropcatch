import { screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import type { ProductSummary } from '@/lib/api/products'
import ProductsPage from '@/routes/products/page'

const airpods: ProductSummary = {
  id: 1,
  name: 'AirPods Pro 3',
  paused: false,
  linkCount: 3,
  bestCents: 22900,
  bestSite: 'shopee',
  typicalCents: 27900,
  pctVsTypical: -17.92,
  spark: [
    { day: '2026-10-16', cents: 27900 },
    { day: '2026-10-17', cents: 22900 },
  ],
  warnings: ['1 link broken'],
}

vi.mock('@/lib/api/client', () => ({
  api: { get: vi.fn(() => Promise.resolve([airpods])) },
}))

describe('ProductsPage', () => {
  it('shows each product with its best price and how it compares to typical', async () => {
    renderWithProviders(
      <MemoryRouter>
        <ProductsPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('AirPods Pro 3')).toBeInTheDocument()
    expect(screen.getByText('$229.00')).toBeInTheDocument()
    expect(screen.getByText('18% under typical')).toBeInTheDocument()
    expect(screen.getByText('1 link broken')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /AirPods Pro 3/ })).toHaveAttribute('href', '/products/1')
  })
})
