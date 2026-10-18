import { Monitor, Moon, Sun } from 'lucide-react'
import { useTheme, type Theme } from '@/app/theme-context'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const OPTIONS: { value: Theme; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  return (
    <ToggleGroup
      type="single"
      size="sm"
      value={theme}
      onValueChange={(value) => {
        if (value) setTheme(value as Theme)
      }}
      className="w-full group-data-[collapsible=icon]:hidden"
    >
      {OPTIONS.map((option) => (
        <ToggleGroupItem key={option.value} value={option.value} aria-label={option.label}>
          <option.icon className="size-4" />
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
