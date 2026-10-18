import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api/client'

export type Settings = {
  dropPct: number
  windowDays: number
  minHistoryDays: number
  checkIntervalHours: number
  /** Masked by the server. Send it back unchanged to keep the stored key. */
  resendApiKey: string
  email: string
}

export type SessionStatus = 'ok' | 'expired' | 'blocked' | 'unknown'
export type Session = { site: string; status: SessionStatus }

export const settingsKeys = {
  all: ['settings'] as const,
  settings: () => [...settingsKeys.all, 'settings'] as const,
  sessions: () => [...settingsKeys.all, 'sessions'] as const,
}

export function settingsQueryOptions() {
  return queryOptions({
    queryKey: settingsKeys.settings(),
    queryFn: () => api.get<Settings>('/settings'),
  })
}

export function sessionsQueryOptions() {
  return queryOptions({
    queryKey: settingsKeys.sessions(),
    queryFn: () => api.get<Session[]>('/sessions'),
    // A login finishes in another window, so poll to notice it.
    refetchInterval: 5000,
  })
}

export function useSaveSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: Settings) => api.put<Settings>('/settings', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
  })
}

export function useSendTestEmail() {
  return useMutation({ mutationFn: () => api.post<null>('/settings/test-email') })
}

export function useOpenLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (site: string) => api.post<null>(`/sessions/${site}/login`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
  })
}
