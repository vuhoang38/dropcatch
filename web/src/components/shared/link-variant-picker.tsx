import { useId, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { usePreviewLink, type Preview } from '@/lib/api/products'
import { formatCents, siteName } from '@/lib/format'

export type PickedLink = { url: string; variantIds: string[]; preview: Preview }

type Props = {
  onPicked: (link: PickedLink | null) => void
}

/**
 * Paste a link, read it once, then tick the variants to track. Used when
 * adding a product and when adding a link to one.
 */
export function LinkVariantPicker({ onPicked }: Props) {
  const inputId = useId()
  const [url, setUrl] = useState('')
  const [preview, setPreview] = useState<Preview | null>(null)
  const [ticked, setTicked] = useState<string[]>([])
  const read = usePreviewLink()

  function pick(next: string[], p: Preview | null = preview) {
    setTicked(next)
    onPicked(p ? { url: p.url, variantIds: next, preview: p } : null)
  }

  async function readLink() {
    setPreview(null)
    onPicked(null)
    const p = await read.mutateAsync(url.trim())
    setPreview(p)
    pick(p.variants.length === 1 ? [p.variants[0].externalId] : [], p)
  }

  function toggle(id: string, on: boolean) {
    pick(on ? [...ticked, id] : ticked.filter((t) => t !== id))
  }

  return (
    <div className="grid gap-3">
      <div className="grid gap-2">
        <Label htmlFor={inputId}>Product link</Label>
        <div className="flex gap-2">
          <Input
            id={inputId}
            value={url}
            placeholder="https://shopee.sg/..."
            onChange={(event) => setUrl(event.target.value)}
          />
          <Button
            variant="outline"
            disabled={url.trim() === '' || read.isPending}
            onClick={() => void readLink().catch(() => undefined)}
          >
            Read
          </Button>
        </div>
        {read.isPending ? (
          <p className="text-caption text-muted-foreground">
            Opening the page in Chrome. This can take up to a minute.
          </p>
        ) : null}
      </div>

      {read.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{read.error.message}</AlertDescription>
        </Alert>
      ) : null}

      {preview ? (
        <div className="grid gap-3 rounded-md border p-3">
          <div className="grid gap-1">
            <span className="text-label">{preview.title}</span>
            <span className="text-caption text-muted-foreground">
              {siteName(preview.site)}
              {preview.seller ? ` · ${preview.seller}` : ''}
            </span>
          </div>

          {preview.variants.length > 1 ? (
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => pick(preview.variants.map((v) => v.externalId))}
              >
                Tick all
              </Button>
              <Button size="sm" variant="outline" onClick={() => pick([])}>
                Untick all
              </Button>
            </div>
          ) : null}

          <ul className="grid max-h-64 gap-2 overflow-y-auto">
            {preview.variants.map((v) => {
              const id = `${inputId}-${v.externalId}`
              return (
                <li key={v.externalId} className="flex items-center gap-2">
                  <Checkbox
                    id={id}
                    checked={ticked.includes(v.externalId)}
                    onCheckedChange={(on) => toggle(v.externalId, on === true)}
                  />
                  <Label htmlFor={id} className="flex-1 text-body">
                    {v.name || 'Default'}
                  </Label>
                  {!v.inStock ? <Badge variant="outline">Out of stock</Badge> : null}
                  <span className="text-body tabular-nums">{formatCents(v.bestCents)}</span>
                </li>
              )
            })}
          </ul>
          {ticked.length === 0 ? (
            <p className="text-caption text-muted-foreground">Tick at least one variant to track.</p>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
