import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { api } from '@/lib/api/client'
import type { History } from '@/lib/api/products'
import { PriceChart } from '@/routes/product/components/price-chart'
import { MAX_DOTS, showDots } from '@/routes/product/components/show-dots'

vi.mock('@/lib/api/client', () => ({
  api: { get: vi.fn() },
}))

const history: History = {
  series: [
    { variantId: 11, label: 'Shopee · Apple Flagship Store · Pro 3', points: [{ t: '2026-10-18T01:41:00Z', cents: 31500 }] },
    { variantId: 12, label: 'Lazada · Apple Flagship Store', points: [{ t: '2026-10-18T01:42:00Z', cents: 32200 }] },
  ],
  typicalCents: null,
}

beforeEach(() => {
  vi.mocked(api.get).mockResolvedValue(history)
})

describe('PriceChart', () => {
  it('lets you hide a line and bring it back', async () => {
    const user = userEvent.setup()
    renderWithProviders(<PriceChart productId={1} />)

    const lazada = await screen.findByRole('checkbox', { name: 'Lazada · Apple Flagship Store' })
    const shopee = screen.getByRole('checkbox', { name: 'Shopee · Apple Flagship Store · Pro 3' })
    expect(lazada).toBeChecked()
    expect(shopee).toBeChecked()

    await user.click(lazada)
    expect(lazada).not.toBeChecked()
    expect(shopee).toBeChecked()

    await user.click(shopee)
    expect(screen.getByText('Tick a line above to see its prices.')).toBeInTheDocument()

    await user.click(lazada)
    expect(lazada).toBeChecked()
    expect(screen.queryByText('Tick a line above to see its prices.')).not.toBeInTheDocument()
  })

  it('shows the last 24 hours', async () => {
    const user = userEvent.setup()
    renderWithProviders(<PriceChart productId={1} />)

    await user.click(await screen.findByRole('radio', { name: '24h' }))
    expect(api.get).toHaveBeenLastCalledWith('/products/1/history', { searchParams: { range: '1d' } })
  })

  it('shows a dot per check until there are too many to tell apart', () => {
    expect(showDots(1)).toBe(true)
    expect(showDots(MAX_DOTS)).toBe(true)
    expect(showDots(MAX_DOTS + 1)).toBe(false)
  })
})
