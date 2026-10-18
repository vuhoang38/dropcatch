import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LinkVariantPicker, type PickedLink } from '@/components/shared/link-variant-picker'
import { useCreateProduct } from '@/lib/api/products'

type Props = { open: boolean; onOpenChange: (open: boolean) => void }

export function AddProductDialog({ open, onOpenChange }: Props) {
  // A fresh form every time it opens.
  const [formKey, setFormKey] = useState(0)

  function change(next: boolean) {
    if (next) setFormKey((k) => k + 1)
    onOpenChange(next)
  }

  return (
    <Dialog open={open} onOpenChange={change}>
      <DialogContent className="max-h-screen overflow-y-auto sm:max-w-2xl">
        <AddProductForm key={formKey} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}

function AddProductForm({ onDone }: { onDone: () => void }) {
  const [name, setName] = useState('')
  const [slots, setSlots] = useState<number[]>([0])
  const [picked, setPicked] = useState<Record<number, PickedLink | null>>({})
  const create = useCreateProduct()

  const links = slots.map((slot) => picked[slot] ?? null)
  const ready =
    name.trim() !== '' && links.length > 0 && links.every((l) => l !== null && l.variantIds.length > 0)

  async function save() {
    await create.mutateAsync({
      name: name.trim(),
      links: links.flatMap((l) => (l ? [{ url: l.url, variantIds: l.variantIds }] : [])),
    })
    toast.success(`Tracking ${name.trim()}.`)
    onDone()
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Add product</DialogTitle>
        <DialogDescription>
          Name it, then paste each listing you want to watch and pick its variants.
        </DialogDescription>
      </DialogHeader>

      <div className="grid gap-2">
        <Label htmlFor="product-name">Name</Label>
        <Input
          id="product-name"
          value={name}
          placeholder="AirPods Pro 3"
          onChange={(event) => setName(event.target.value)}
        />
      </div>

      {slots.map((slot, index) => (
        <div key={slot} className="grid gap-2">
          <div className="flex items-center justify-between">
            <span className="text-label">Link {index + 1}</span>
            {slots.length > 1 ? (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setSlots((s) => s.filter((x) => x !== slot))}
              >
                Remove
              </Button>
            ) : null}
          </div>
          <LinkVariantPicker onPicked={(l) => setPicked((p) => ({ ...p, [slot]: l }))} />
        </div>
      ))}

      <Button
        variant="outline"
        onClick={() => setSlots((s) => [...s, (s.at(-1) ?? 0) + 1])}
      >
        Add another link
      </Button>

      {create.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{create.error.message}</AlertDescription>
        </Alert>
      ) : null}

      <DialogFooter>
        <Button variant="outline" onClick={onDone}>
          Cancel
        </Button>
        <Button disabled={!ready || create.isPending} onClick={() => void save().catch(() => undefined)}>
          {create.isPending ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </>
  )
}
