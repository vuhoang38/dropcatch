import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { PageHeader } from '@/components/shared/page-header'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  sessionsQueryOptions,
  settingsQueryOptions,
  useOpenLogin,
  useSaveSettings,
  useSendTestEmail,
  type SessionStatus,
  type Settings,
} from '@/lib/api/settings'
import { siteName } from '@/lib/format'

export default function SettingsPage() {
  const { data, isPending } = useQuery(settingsQueryOptions())

  return (
    <>
      <PageHeader title="Settings" description="Alert defaults, email, and the logins Dropcatch uses." />
      {isPending || !data ? (
        <Skeleton className="h-96 w-full" />
      ) : (
        <SettingsForm key={JSON.stringify(data)} settings={data} />
      )}
      <Logins />
    </>
  )
}

type NumberField = 'dropPct' | 'windowDays' | 'minHistoryDays' | 'checkIntervalHours'

const NUMBER_FIELDS: { key: NumberField; label: string; unit: string; hint: string }[] = [
  { key: 'dropPct', label: 'Alert when this % under typical', unit: '%', hint: 'Default for every product.' },
  { key: 'windowDays', label: 'Typical price window', unit: 'days', hint: 'Typical price is the median of these days.' },
  { key: 'minHistoryDays', label: 'Days of history before alerting', unit: 'days', hint: 'No alerts before this.' },
  { key: 'checkIntervalHours', label: 'Check prices every', unit: 'hours', hint: 'Runs while your laptop is awake.' },
]

function SettingsForm({ settings }: { settings: Settings }) {
  const [form, setForm] = useState(settings)
  const save = useSaveSettings()
  const testEmail = useSendTestEmail()

  async function submit() {
    await save.mutateAsync(form)
    toast.success('Settings saved.')
  }

  return (
    <div className="grid gap-6">
      <section className="grid gap-4 rounded-md border p-4">
        <span className="text-h3">Alerts</span>
        <div className="grid gap-4 sm:grid-cols-2">
          {NUMBER_FIELDS.map((f) => (
            <div key={f.key} className="grid content-start gap-2">
              <Label htmlFor={`setting-${f.key}`}>{f.label}</Label>
              <div className="flex items-center gap-2">
                <Input
                  id={`setting-${f.key}`}
                  type="number"
                  min={0}
                  value={form[f.key]}
                  onChange={(event) => setForm((s) => ({ ...s, [f.key]: Number(event.target.value) }))}
                />
                <span className="text-caption text-muted-foreground">{f.unit}</span>
              </div>
              <span className="text-caption text-muted-foreground">{f.hint}</span>
            </div>
          ))}
        </div>
      </section>

      <section className="grid gap-4 rounded-md border p-4">
        <div className="grid gap-1">
          <span className="text-h3">Email</span>
          <span className="text-caption text-muted-foreground">
            Dropcatch sends through Resend. Make a free account at resend.com, create an API key,
            and use the email address you signed up with.
          </span>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="grid gap-2">
            <Label htmlFor="setting-key">Resend API key</Label>
            <Input
              id="setting-key"
              type="password"
              autoComplete="off"
              value={form.resendApiKey}
              placeholder="re_..."
              onChange={(event) => setForm((s) => ({ ...s, resendApiKey: event.target.value }))}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="setting-email">Send alerts to</Label>
            <Input
              id="setting-email"
              type="email"
              value={form.email}
              placeholder="you@example.com"
              onChange={(event) => setForm((s) => ({ ...s, email: event.target.value }))}
            />
          </div>
        </div>
        <Button
          variant="outline"
          className="justify-self-start"
          disabled={testEmail.isPending}
          onClick={() =>
            testEmail.mutate(undefined, {
              onSuccess: () => toast.success('Test email sent. Check your inbox.'),
              onError: (error) => toast.error(error.message),
            })
          }
        >
          Send test email
        </Button>
        <span className="text-caption text-muted-foreground">Save first if you just changed the key or email.</span>
      </section>

      {save.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{save.error.message}</AlertDescription>
        </Alert>
      ) : null}
      <Button
        className="justify-self-start"
        aria-label="Save settings"
        disabled={save.isPending}
        onClick={() => void submit().catch(() => undefined)}
      >
        Save
      </Button>
    </div>
  )
}

const STATUS: Record<SessionStatus, { text: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  ok: { text: 'Logged in', variant: 'secondary' },
  expired: { text: 'Login expired', variant: 'destructive' },
  blocked: { text: 'Needs verification', variant: 'destructive' },
  unknown: { text: 'Not checked yet', variant: 'outline' },
}

function Logins() {
  const { data } = useQuery(sessionsQueryOptions())
  const login = useOpenLogin()

  return (
    <section className="grid gap-4 rounded-md border p-4">
      <div className="grid gap-1">
        <span className="text-h3">Logins</span>
        <span className="text-caption text-muted-foreground">
          Shopee and Lazada only show your vouchers when you're logged in. Dropcatch uses its own
          Chrome profile, so log in there once.
        </span>
      </div>
      <ul className="grid gap-3">
        {(data ?? []).map((s) => {
          const action = s.status === 'ok' ? 'Log in again' : 'Log in'
          return (
            <li key={s.site} className="flex flex-wrap items-center gap-3">
              <span className="text-label w-24">{siteName(s.site)}</span>
              <Badge variant={STATUS[s.status].variant}>{STATUS[s.status].text}</Badge>
              <Button
                size="sm"
                variant="outline"
                className="ml-auto"
                aria-label={`${action} to ${siteName(s.site)}`}
                disabled={login.isPending}
                onClick={() =>
                  login.mutate(s.site, {
                    onSuccess: () => toast.success('A Chrome window opened. Log in there. It hides itself once you are in.'),
                    onError: (error) => toast.error(error.message),
                  })
                }
              >
                {action}
              </Button>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
