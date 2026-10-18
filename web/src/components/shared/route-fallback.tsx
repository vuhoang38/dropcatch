import { Skeleton } from '@/components/ui/skeleton'

/** Shown while a lazily loaded route chunk is still downloading. */
export function RouteFallback() {
  return (
    <div className="flex flex-col gap-6">
      <Skeleton className="h-12 w-64" />
      <Skeleton className="h-64 w-full" />
    </div>
  )
}
