import { describe, expect, it } from 'vitest'
import { sessionsQueryOptions } from '@/lib/api/settings'

describe('sessionsQueryOptions', () => {
  it('refreshes login status on its own, so a finished login shows up', () => {
    expect(sessionsQueryOptions().refetchInterval).toBe(5000)
  })
})
