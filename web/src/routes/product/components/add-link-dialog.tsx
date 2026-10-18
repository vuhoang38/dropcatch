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
import { LinkVariantPicker, type PickedLink } from '@/components/shared/link-variant-picker'
import { useAddLink } from '@/lib/api/products'

type Props = { productId: number; open: boolean; onOpenChange: (open: boolean) => void }

export function AddLinkDialog({ productId, open, onOpenChange }: Props) {
  const [formKey, setFormKey] = useState(0)
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) setFormKey((k) => k + 1)
        onOpenChange(next)
      }}
    >
      <DialogContent className="max-h-screen overflow-y-auto sm:max-w-2xl">
        <AddLinkForm key={formKey} productId={productId} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}

function AddLinkForm({ productId, onDone }: { productId: number; onDone: () => void }) {
  const [picked, setPicked] = useState<PickedLink | null>(null)
  const add = useAddLink(productId)

  async function save() {
    if (!picked) return
    await add.mutateAsync({ url: picked.url, variantIds: picked.variantIds })
    toast.success('Link added.')
    onDone()
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Add link</DialogTitle>
        <DialogDescription>Paste another listing for this product and pick its variants.</DialogDescription>
      </DialogHeader>
      <LinkVariantPicker onPicked={setPicked} />
      {add.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{add.error.message}</AlertDescription>
        </Alert>
      ) : null}
      <DialogFooter>
        <Button variant="outline" onClick={onDone}>
          Cancel
        </Button>
        <Button
          disabled={!picked || picked.variantIds.length === 0 || add.isPending}
          onClick={() => void save().catch(() => undefined)}
        >
          {add.isPending ? 'Saving...' : 'Add link'}
        </Button>
      </DialogFooter>
    </>
  )
}
