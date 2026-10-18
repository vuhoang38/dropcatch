import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api/client'

export type SparkPoint = { day: string; cents: number }

export type ProductSummary = {
  id: number
  name: string
  paused: boolean
  linkCount: number
  bestCents: number | null
  bestSite: string
  typicalCents: number | null
  /** (best - typical) / typical * 100. Negative means cheaper than typical. */
  pctVsTypical: number | null
  spark: SparkPoint[]
  warnings: string[]
}

export type AlertSettings = { dropPct: number; windowDays: number; minHistoryDays: number }

export type Overrides = {
  dropPct: number | null
  windowDays: number | null
  minHistoryDays: number | null
}

export type VariantDetail = {
  id: number
  externalId: string
  name: string
  tracked: boolean
  isNew: boolean
  gone: boolean
  inStock: boolean | null
  priceCents: number | null
  bestCents: number | null
  bestIsEstimate: boolean
  originalCents: number | null
  checkedAt: string | null
}

export type LinkStatus = 'ok' | 'broken' | 'pending'

export type LinkDetail = {
  id: number
  url: string
  site: string
  title: string
  seller: string
  status: LinkStatus
  lastError: string
  lastCheckedAt: string | null
  variants: VariantDetail[]
}

export type ProductDetail = ProductSummary & {
  settings: AlertSettings
  overrides: Overrides
  links: LinkDetail[]
}

export type HistoryRange = '7d' | '30d' | '90d' | 'all'

export type History = {
  series: { variantId: number; label: string; points: { t: string; cents: number }[] }[]
  typicalCents: number | null
}

export type PreviewVariant = {
  externalId: string
  name: string
  priceCents: number
  bestCents: number
  bestIsEstimate: boolean
  inStock: boolean
}

export type Preview = {
  site: string
  url: string
  title: string
  seller: string
  variants: PreviewVariant[]
}

export type LinkInput = { url: string; variantIds: string[] }

export const productKeys = {
  all: ['products'] as const,
  list: () => [...productKeys.all, 'list'] as const,
  detail: (id: number) => [...productKeys.all, 'detail', id] as const,
  history: (id: number, range: HistoryRange) => [...productKeys.all, 'history', id, range] as const,
}

export function productsQueryOptions() {
  return queryOptions({
    queryKey: productKeys.list(),
    queryFn: () => api.get<ProductSummary[]>('/products'),
  })
}

export function productQueryOptions(id: number) {
  return queryOptions({
    queryKey: productKeys.detail(id),
    queryFn: () => api.get<ProductDetail>(`/products/${id}`),
  })
}

export function historyQueryOptions(id: number, range: HistoryRange) {
  return queryOptions({
    queryKey: productKeys.history(id, range),
    queryFn: () => api.get<History>(`/products/${id}/history`, { searchParams: { range } }),
  })
}

function useInvalidateProducts() {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: productKeys.all })
}

export function useCreateProduct() {
  const invalidate = useInvalidateProducts()
  return useMutation({
    mutationFn: (input: { name: string; links: LinkInput[] }) =>
      api.post<ProductDetail>('/products', input),
    onSuccess: invalidate,
  })
}

export type ProductPatch = {
  name?: string
  paused?: boolean
  dropPct?: number | null
  windowDays?: number | null
  minHistoryDays?: number | null
}

export function useUpdateProduct(id: number) {
  const invalidate = useInvalidateProducts()
  return useMutation({
    mutationFn: (patch: ProductPatch) => api.patch<ProductDetail>(`/products/${id}`, patch),
    onSuccess: invalidate,
  })
}

export function useDeleteProduct() {
  const invalidate = useInvalidateProducts()
  return useMutation({
    mutationFn: (id: number) => api.delete<null>(`/products/${id}`),
    onSuccess: invalidate,
  })
}

export function useAddLink(productId: number) {
  const invalidate = useInvalidateProducts()
  return useMutation({
    mutationFn: (input: LinkInput) => api.post<ProductDetail>(`/products/${productId}/links`, input),
    onSuccess: invalidate,
  })
}

export function useDeleteLink() {
  const invalidate = useInvalidateProducts()
  return useMutation({
    mutationFn: (linkId: number) => api.delete<null>(`/links/${linkId}`),
    onSuccess: invalidate,
  })
}

export function useSetVariantTracked() {
  const invalidate = useInvalidateProducts()
  return useMutation({
    mutationFn: ({ id, tracked }: { id: number; tracked: boolean }) =>
      api.patch<null>(`/variants/${id}`, { tracked }),
    onSuccess: invalidate,
  })
}

export function useCheckNow(productId: number) {
  return useMutation({
    mutationFn: () => api.post<null>(`/products/${productId}/check`),
  })
}

export function usePreviewLink() {
  return useMutation({
    mutationFn: (url: string) => api.post<Preview>('/preview', { url }),
  })
}
