import { useState } from "react"
import { ChevronsUpDown, Check, Plus, Building2 } from "lucide-react"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/components/ui/sidebar"
import { CreateGroupDialog } from "@/views/MembersView"
import { MOCK_GROUPS } from "@/data/mock"

interface LocationGroupSwitcherProps {
  activeGroupId: string
  onGroupChange: (groupId: string) => void
}

export function LocationGroupSwitcher({ activeGroupId, onGroupChange }: LocationGroupSwitcherProps) {
  const [open, setOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const activeGroup = MOCK_GROUPS.find((g) => g.id === activeGroupId) ?? MOCK_GROUPS[0]

  return (
    <>
      <SidebarMenu>
        <SidebarMenuItem>
          <DropdownMenu open={open} onOpenChange={setOpen}>
            <DropdownMenuTrigger asChild>
              <SidebarMenuButton
                size="lg"
                className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
              >
                <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shrink-0">
                  <Building2 className="size-4" />
                </div>
                <div className="flex flex-col gap-0.5 leading-none min-w-0">
                  <span className="font-semibold text-sm truncate">{activeGroup.name}</span>
                  <span className="text-xs text-muted-foreground truncate">
                    {activeGroup.members.length} member{activeGroup.members.length !== 1 ? "s" : ""}
                  </span>
                </div>
                <ChevronsUpDown className="ml-auto size-4 shrink-0 text-muted-foreground" />
              </SidebarMenuButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              className="w-[--radix-dropdown-menu-trigger-width] min-w-56"
              align="start"
              side="bottom"
              sideOffset={4}
            >
              {MOCK_GROUPS.map((group) => (
                <DropdownMenuItem
                  key={group.id}
                  onSelect={() => { onGroupChange(group.id); setOpen(false) }}
                  className="gap-2 p-2"
                >
                  <div className="flex size-6 items-center justify-center rounded-md bg-primary/10 shrink-0">
                    <Building2 className="size-3.5 text-primary" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{group.name}</p>
                    <p className="text-xs text-muted-foreground truncate">{group.description}</p>
                  </div>
                  {group.id === activeGroupId && (
                    <Check className="size-4 text-primary shrink-0" />
                  )}
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="gap-2 p-2 text-muted-foreground"
                onSelect={() => { setOpen(false); setCreateOpen(true) }}
              >
                <div className="flex size-6 items-center justify-center rounded-md border border-dashed border-border">
                  <Plus className="size-3.5" />
                </div>
                <span className="text-sm">Create new group</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarMenuItem>
      </SidebarMenu>

      <CreateGroupDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(name) => console.log("Created group:", name)}
      />
    </>
  )
}
