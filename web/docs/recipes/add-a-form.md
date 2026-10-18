# Add a form

## When

Collecting input that needs validation.

## Files

```
src/routes/<name>/page.tsx                      or
src/routes/<name>/components/<thing>-form.tsx   when the form is reused or long
```

Stack: `react-hook-form` for state, `zod` for the schema, shadcn `Form` for the
markup.

## Pattern

```tsx
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

const schema = z.object({
  name: z.string().min(2, 'Use at least 2 characters.'),
  email: z.string().email('Enter a valid email address.'),
})

type Values = z.infer<typeof schema>

export function ReportForm() {
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', email: '' },
  })

  function onSubmit(values: Values) {
    toast.success(`Saved ${values.name}.`)
  }

  return (
    <Form {...form}>
      <form
        noValidate
        onSubmit={(event) => void form.handleSubmit(onSubmit)(event)}
        className="flex max-w-2xl flex-col gap-6"
      >
        <FormField
          control={form.control}
          name="name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Name</FormLabel>
              <FormControl>
                <Input {...field} />
              </FormControl>
              <FormDescription>Shown to everyone in the workspace.</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button type="submit" disabled={form.formState.isSubmitting}>
          Save
        </Button>
      </form>
    </Form>
  )
}
```

Working examples: `src/routes/settings/page.tsx`, `src/routes/login/page.tsx`.

## Mistakes that happen here

- **Leaving `noValidate` off the `<form>`.** With `<Input type="email">` the
  browser blocks submission on its own and shows a native bubble, so your zod
  message never appears and the submit handler never runs. This one is easy to
  miss because the page looks fine until a field is invalid.
- **Writing `onSubmit={form.handleSubmit(onSubmit)}`.** It returns a promise
  where a void return is expected, which lint rejects. Wrap it as above.
- **Using a plain `<input>` or omitting `FormControl`.** The label then stops
  pointing at the field, which breaks both accessibility and
  `getByLabelText` in tests.
- **Dropping `FormMessage`,** which means validation fails silently.
- **Calling `setState` in an effect to sync a form with props.** Key the
  component on the record id instead, as `change-role-dialog.tsx` does.

## Testing a form

```tsx
const user = userEvent.setup()
renderWithProviders(<SettingsPage />)

await user.clear(screen.getByLabelText('Support email'))
await user.type(screen.getByLabelText('Support email'), 'not-an-email')
await user.click(screen.getByRole('button', { name: 'Save changes' }))

expect(await screen.findByText('Enter a valid email address.')).toBeInTheDocument()
```
