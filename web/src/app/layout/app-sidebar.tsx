import { Package, Settings, TrendingDown } from 'lucide-react'
import { NavLink } from 'react-router'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from '@/components/ui/sidebar'
import { ThemeToggle } from '@/app/layout/theme-toggle'

const NAV_ITEMS = [
  { title: 'Products', to: '/', icon: Package, end: true },
  { title: 'Settings', to: '/settings', icon: Settings, end: false },
]

export function AppSidebar() {
  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        {/*
          Collapsed, the sidebar is 3rem and SidebarHeader already spends p-2
          of it. Keeping px-2 here too would leave 1rem of box for a size-8
          logo, pushing it flush against the divider. Drop the inset and the
          header padding alone lands the logo on the same x as the nav icons.
        */}
        <div className="flex items-center gap-2 px-2 py-1 group-data-[collapsible=icon]:px-0">
          <div className="bg-primary text-primary-foreground flex size-8 shrink-0 items-center justify-center rounded-md">
            <TrendingDown className="size-4" />
          </div>
          <div className="grid group-data-[collapsible=icon]:hidden">
            <span className="text-label truncate">Dropcatch</span>
            <span className="text-caption text-muted-foreground truncate">Price tracker</span>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Tracker</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV_ITEMS.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <NavLink to={item.to} end={item.end}>
                    {({ isActive }) => (
                      <SidebarMenuButton isActive={isActive} tooltip={item.title}>
                        <item.icon />
                        <span>{item.title}</span>
                      </SidebarMenuButton>
                    )}
                  </NavLink>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter>
        <ThemeToggle />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
