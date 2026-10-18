import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { api } from '@/lib/api/client'
import type { Session, Settings } from '@/lib/api/settings'
import SettingsPage from '@/routes/settings/page'

vi.mock('@/lib/api/client', () => ({ api: { get: vi.fn(), put: vi.fn(), post: vi.fn() } }))

const settings: Settings = {
  dropPct: 20,
  windowDays: 30,
  minHistoryDays: 2,
  checkIntervalHours: 3,
  resendApiKey: '••••1234',
  email: 'me@x.com',
}
const sessions: Session[] = [
  { site: 'shopee', status: 'ok' },
  { site: 'lazada', status: 'expired' },
]

beforeEach(() => {
  vi.mocked(api.get).mockImplementation((path: string) =>
    Promise.resolve(path === '/sessions' ? sessions : settings),
  )
  vi.mocked(api.put).mockImplementation((_path: string, body: unknown) => Promise.resolve(body))
  vi.mocked(api.post).mockResolvedValue(null)
})

describe('SettingsPage', () => {
  it('saves a changed drop % and sends the masked key back untouched', async () => {
    renderWithProviders(<SettingsPage />)
    const drop = await screen.findByLabelText('Alert when this % under typical')
    await userEvent.clear(drop)
    await userEvent.type(drop, '25')
    await userEvent.click(screen.getByRole('button', { name: 'Save settings' }))

    expect(api.put).toHaveBeenCalledWith('/settings', { ...settings, dropPct: 25 })
  })

  it('shows each login and opens a login window on request', async () => {
    renderWithProviders(<SettingsPage />)

    expect(await screen.findByText('Login expired')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Log in to Lazada' }))

    expect(api.post).toHaveBeenCalledWith('/sessions/lazada/login')
  })

  it('sends a test email', async () => {
    renderWithProviders(<SettingsPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Send test email' }))

    expect(api.post).toHaveBeenCalledWith('/settings/test-email')
  })
})
