import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

const tv: ProductSummary = { ...airpods, id: 2, name: 'Sony Bravia TV', bestCents: 99900, pctVsTypical: null, typicalCents: null, warnings: [] }

vi.mock('@/lib/api/client', () => ({
  api: { get: vi.fn(() => Promise.resolve([airpods, tv])) },
}))

function renderPage() {
  return renderWithProviders(
    <MemoryRouter>
      <ProductsPage />
    </MemoryRouter>,
  )
}

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

  it('narrows the list to products whose name matches your search', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('AirPods Pro 3')

    await user.type(screen.getByRole('searchbox', { name: 'Search products' }), 'airpods')
    expect(screen.getByText('AirPods Pro 3')).toBeInTheDocument()
    expect(screen.queryByText('Sony Bravia TV')).not.toBeInTheDocument()

    await user.clear(screen.getByRole('searchbox', { name: 'Search products' }))
    await user.type(screen.getByRole('searchbox', { name: 'Search products' }), 'kettle')
    expect(screen.getByText('No products match "kettle".')).toBeInTheDocument()
  })
})
