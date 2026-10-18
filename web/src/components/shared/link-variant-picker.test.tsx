import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { api } from '@/lib/api/client'
import type { Preview } from '@/lib/api/products'
import { LinkVariantPicker } from '@/components/shared/link-variant-picker'

vi.mock('@/lib/api/client', () => ({ api: { post: vi.fn() } }))

const twoVariants: Preview = {
  site: 'shopee',
  url: 'https://shopee.sg/product/1/2',
  title: 'AirPods case',
  seller: 'Case Shop',
  variants: [
    { externalId: 'a', name: 'Black', priceCents: 700, bestCents: 680, bestIsEstimate: false, inStock: true },
    { externalId: 'b', name: 'Purple', priceCents: 700, bestCents: 700, bestIsEstimate: true, inStock: true },
  ],
}

async function readLink(preview: Preview) {
  vi.mocked(api.post).mockResolvedValue(preview)
  const onPicked = vi.fn()
  renderWithProviders(<LinkVariantPicker onPicked={onPicked} />)
  await userEvent.type(screen.getByLabelText('Product link'), 'https://shopee.sg/x')
  await userEvent.click(screen.getByRole('button', { name: 'Read' }))
  await screen.findByText('AirPods case')
  return onPicked
}

describe('LinkVariantPicker', () => {
  it('lists every variant and ticks them all on request', async () => {
    const onPicked = await readLink(twoVariants)

    expect(screen.getAllByRole('checkbox')).toHaveLength(2)
    await userEvent.click(screen.getByRole('button', { name: 'Tick all' }))

    expect(onPicked).toHaveBeenLastCalledWith({
      url: 'https://shopee.sg/product/1/2',
      variantIds: ['a', 'b'],
      preview: twoVariants,
    })
  })

  it('pre-ticks a listing with a single variant', async () => {
    const one = { ...twoVariants, variants: [twoVariants.variants[0]] }
    const onPicked = await readLink(one)

    expect(screen.getByRole('checkbox')).toBeChecked()
    expect(onPicked).toHaveBeenLastCalledWith({ url: one.url, variantIds: ['a'], preview: one })
  })

  it('shows the server message when a link cannot be read', async () => {
    vi.mocked(api.post).mockRejectedValue(new Error("You're logged out of this site."))
    renderWithProviders(<LinkVariantPicker onPicked={vi.fn()} />)
    await userEvent.type(screen.getByLabelText('Product link'), 'https://shopee.sg/x')
    await userEvent.click(screen.getByRole('button', { name: 'Read' }))

    expect(await screen.findByText("You're logged out of this site.")).toBeInTheDocument()
  })
})
