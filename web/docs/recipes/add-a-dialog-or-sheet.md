# Add a dialog or sheet

## When

A task belongs in an overlay rather than its own page. `Dialog` for a short
focused task, `Sheet` for a side panel with more in it.

## Files

```
src/routes/<name>/components/<action>-dialog.tsx
```

## The shape that avoids the usual bug

An overlay that edits a record needs its state to start from that record. Doing
that with an effect causes a cascading render and lint rejects it. Key the inner
component on the record id instead, and let the parent hold `open` separately
from the record so content survives the close animation.

```tsx
type Props = {
  open: boolean
  report: Report | null
  onOpenChange: (open: boolean) => void
}

export function RenameReportDialog({ open, report, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {report ? (
          <RenameForm key={report.id} report={report} onDone={() => onOpenChange(false)} />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function RenameForm({ report, onDone }: { report: Report; onDone: () => void }) {
  const [name, setName] = useState(report.name)
  const rename = useRenameReport()

  async function save() {
    await rename.mutateAsync({ id: report.id, name })
    toast.success('Renamed.')
    onDone()
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Rename report</DialogTitle>
        <DialogDescription>Give {report.name} a new name.</DialogDescription>
      </DialogHeader>

      <div className="grid gap-2">
        <Label htmlFor="name">Name</Label>
        <Input id="name" value={name} onChange={(event) => setName(event.target.value)} />
      </div>

      <DialogFooter>
        <Button variant="outline" onClick={onDone}>Cancel</Button>
        <Button onClick={() => void save()} disabled={rename.isPending}>
          {rename.isPending ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </>
  )
}
```

In the page:

```tsx
const [editing, setEditing] = useState<Report | null>(null)
const [dialogOpen, setDialogOpen] = useState(false)

// open it
setEditing(report)
setDialogOpen(true)

<RenameReportDialog open={dialogOpen} report={editing} onOpenChange={setDialogOpen} />
```

Working example: `src/routes/users/components/change-role-dialog.tsx`.

## Mistakes that happen here

- **`useEffect` to copy props into state.** Lint rejects it and it causes an
  extra render. Use the `key` above.
- **Nulling the record on close.** The dialog then empties while it animates
  out. Keep `open` and the record separate.
- **Omitting `DialogTitle`.** Radix warns, and screen readers get an unnamed
  dialog. If the title should not be visible, wrap it in `VisuallyHidden`.
- **Calling an async handler directly in `onClick`,** which lint rejects. Use
  `onClick={() => void save()}`.

## Testing an overlay

jsdom has none of the browser APIs Radix needs. `src/test/setup.ts` already
polyfills `ResizeObserver`, `matchMedia` and the pointer capture methods, so
tests that open a dialog work without extra setup.
