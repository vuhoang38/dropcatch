import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useUpdateProduct, type ProductDetail } from '@/lib/api/products'

type Field = 'dropPct' | 'windowDays' | 'minHistoryDays'

const FIELDS: { key: Field; label: string; short: string; unit: string }[] = [
  { key: 'dropPct', label: 'Alert when this % under typical', short: 'drop %', unit: '%' },
  { key: 'windowDays', label: 'Typical price window', short: 'window', unit: 'days' },
  { key: 'minHistoryDays', label: 'Days of history before alerting', short: 'minimum history', unit: 'days' },
]

/** Empty means "use the global default from Settings". */
export function AlertSettingsForm({ product }: { product: ProductDetail }) {
  const [values, setValues] = useState<Record<Field, string>>({
    dropPct: product.overrides.dropPct?.toString() ?? '',
    windowDays: product.overrides.windowDays?.toString() ?? '',
    minHistoryDays: product.overrides.minHistoryDays?.toString() ?? '',
  })
  const update = useUpdateProduct(product.id)

  function value(key: Field) {
    return values[key].trim() === '' ? null : Number(values[key])
  }

  async function save() {
    await update.mutateAsync({
      dropPct: value('dropPct'),
      windowDays: value('windowDays'),
      minHistoryDays: value('minHistoryDays'),
    })
    toast.success('Alert settings saved.')
  }

  return (
    <div className="grid gap-4 rounded-md border p-4">
      <div className="grid gap-1">
        <span className="text-label">Alert settings</span>
        <span className="text-caption text-muted-foreground">
          Leave a field empty to use the default from Settings.
        </span>
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        {FIELDS.map((f) => (
          <div key={f.key} className="grid content-start gap-2">
            <Label htmlFor={`alert-${f.key}`}>{f.label}</Label>
            <div className="flex items-center gap-2">
              <Input
                id={`alert-${f.key}`}
                type="number"
                min={0}
                value={values[f.key]}
                placeholder={`default (${product.settings[f.key]})`}
                onChange={(event) => setValues((v) => ({ ...v, [f.key]: event.target.value }))}
              />
              <span className="text-caption text-muted-foreground">{f.unit}</span>
            </div>
            <Button
              size="sm"
              variant="ghost"
              className="justify-self-start"
              aria-label={`Use default ${f.short}`}
              disabled={values[f.key] === ''}
              onClick={() => setValues((v) => ({ ...v, [f.key]: '' }))}
            >
              Use default
            </Button>
          </div>
        ))}
      </div>
      {update.isError ? <p className="text-caption text-destructive">{update.error.message}</p> : null}
      <Button
        className="justify-self-start"
        aria-label="Save alert settings"
        disabled={update.isPending}
        onClick={() => void save().catch(() => undefined)}
      >
        Save
      </Button>
    </div>
  )
}
