/**
 * The only place in the app that talks to the network.
 *
 * Point VITE_API_URL at your real backend and every call follows. Nothing else
 * should call `fetch` directly.
 */

const BASE_URL = import.meta.env.VITE_API_URL ?? '/api'

export class ApiError extends Error {
  readonly status: number
  readonly body: unknown

  constructor(status: number, message: string, body: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

type QueryValue = string | number | boolean | undefined | null

type RequestOptions = {
  signal?: AbortSignal
  searchParams?: Record<string, QueryValue>
  headers?: Record<string, string>
}

function buildUrl(path: string, searchParams?: Record<string, QueryValue>) {
  const url = `${BASE_URL}${path.startsWith('/') ? path : `/${path}`}`
  if (!searchParams) return url

  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(searchParams)) {
    if (value !== undefined && value !== null) params.set(key, String(value))
  }
  const query = params.toString()
  return query ? `${url}?${query}` : url
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: RequestOptions = {},
): Promise<T> {
  const response = await fetch(buildUrl(path, options.searchParams), {
    method,
    signal: options.signal,
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...options.headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  const payload: unknown = response.status === 204 ? null : await response.json().catch(() => null)

  if (!response.ok) {
    const message =
      typeof payload === 'object' && payload !== null && 'message' in payload
        ? String(payload.message)
        : response.statusText
    throw new ApiError(response.status, message, payload)
  }

  return payload as T
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) => request<T>('GET', path, undefined, options),
  post: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>('POST', path, body, options),
  patch: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>('PATCH', path, body, options),
  put: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>('PUT', path, body, options),
  delete: <T>(path: string, options?: RequestOptions) =>
    request<T>('DELETE', path, undefined, options),
}
