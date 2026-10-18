import { formatDistanceToNow } from 'date-fns'
import { ExternalLink, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { useDeleteLink, useSetVariantTracked, type LinkDetail } from '@/lib/api/products'
import { formatCents, siteName } from '@/lib/format'

export function LinksTable({ links }: { links: LinkDetail[] }) {
  const setTracked = useSetVariantTracked()
  const deleteLink = useDeleteLink()

  if (links.length === 0) {
    return <p className="text-body text-muted-foreground">No links yet. Add one to start tracking.</p>
  }

  return (
    <div className="grid gap-3">
      {links.map((link) => (
        <div key={link.id} className="grid gap-3 rounded-md border p-4">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="grid min-w-0 gap-1">
              <span className="text-label">
                {siteName(link.site)}
                {link.seller ? ` · ${link.seller}` : ''}
              </span>
              <span className="text-caption text-muted-foreground truncate">{link.title}</span>
              <span className="text-caption text-muted-foreground">
                {link.lastCheckedAt
                  ? `Checked ${formatDistanceToNow(new Date(link.lastCheckedAt), { addSuffix: true })}`
                  : 'Not checked yet'}
              </span>
            </div>
            <div className="flex items-center gap-2">
              {link.status === 'broken' ? <Badge variant="destructive">Broken</Badge> : null}
              <Button variant="ghost" size="icon" asChild>
                <a href={link.url} target="_blank" rel="noreferrer" aria-label="Open listing">
                  <ExternalLink />
                </a>
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Remove link"
                onClick={() => {
                  if (window.confirm(`Stop tracking this ${siteName(link.site)} link? Its history goes too.`)) {
                    deleteLink.mutate(link.id)
                  }
                }}
              >
                <Trash2 />
              </Button>
            </div>
          </div>

          {link.status === 'broken' && link.lastError ? (
            <p className="text-caption text-destructive">{link.lastError}</p>
          ) : null}

          <ul className="grid max-h-80 gap-2 overflow-y-auto">
            {link.variants.map((v) => {
              const id = `variant-${v.id}`
              const name = v.name || 'Default'
              return (
                <li key={v.id} className="flex flex-wrap items-center gap-2">
                  <Checkbox
                    id={id}
                    aria-label={`Track ${name}`}
                    checked={v.tracked}
                    onCheckedChange={(on) => setTracked.mutate({ id: v.id, tracked: on === true })}
                  />
                  <label htmlFor={id} className="text-body min-w-0 flex-1 truncate">
                    {name}
                  </label>
                  {v.isNew ? <Badge>New</Badge> : null}
                  {v.gone ? <Badge variant="secondary">Gone</Badge> : null}
                  {v.inStock === false ? <Badge variant="outline">Out of stock</Badge> : null}
                  {v.bestCents !== null ? (
                    <span className="grid text-right">
                      <span className="text-body tabular-nums">{formatCents(v.bestCents)}</span>
                      <span className="text-caption text-muted-foreground">
                        {v.bestIsEstimate
                          ? 'no voucher price shown'
                          : v.priceCents !== null && v.priceCents !== v.bestCents
                            ? `${formatCents(v.priceCents)} before vouchers`
                            : v.originalCents
                              ? `"was" ${formatCents(v.originalCents)}`
                              : ''}
                      </span>
                    </span>
                  ) : (
                    <span className="text-caption text-muted-foreground">not tracked yet</span>
                  )}
                </li>
              )
            })}
          </ul>
        </div>
      ))}
    </div>
  )
}
