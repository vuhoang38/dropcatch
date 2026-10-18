import { screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { api } from '@/lib/api/client'
import { Breadcrumbs } from '@/app/layout/breadcrumbs'

vi.mock('@/lib/api/client', () => ({
  api: { get: vi.fn(), patch: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => {
  vi.mocked(api.get).mockResolvedValue({ id: 1, name: 'AirPods Pro 3' })
})

function renderAt(path: string) {
  return renderWithProviders(
    <MemoryRouter initialEntries={[path]}>
      <Breadcrumbs />
    </MemoryRouter>,
  )
}

describe('Breadcrumbs', () => {
  it('names the product you are looking at', async () => {
    renderAt('/products/1')

    expect(await screen.findByText('AirPods Pro 3')).toBeInTheDocument()
    expect(api.get).toHaveBeenCalledWith('/products/1')
    expect(screen.getByRole('link', { name: 'Products' })).toHaveAttribute('href', '/')
  })

  it('says Product until the name loads', () => {
    vi.mocked(api.get).mockReturnValue(new Promise(() => {}))
    renderAt('/products/1')

    expect(screen.getByText('Product')).toBeInTheDocument()
  })

  it('labels the settings page', () => {
    renderAt('/settings')

    expect(screen.getByText('Settings')).toBeInTheDocument()
    expect(api.get).not.toHaveBeenCalled()
  })
})
