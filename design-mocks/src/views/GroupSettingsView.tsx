import { useState } from "react"
import { Users, HardDriveDownload, ChevronRight, Trash2, Info, Zap, Package, FileText, MapPin } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { MOCK_GROUPS } from "@/data/mock"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"

interface GroupSettingsViewProps {
  activeGroupId: string
  onNavigate?: (view: string) => void
}

export function GroupSettingsView({ activeGroupId, onNavigate }: GroupSettingsViewProps) {
  const group = MOCK_GROUPS.find((g) => g.id === activeGroupId) ?? MOCK_GROUPS[0]

  const [groupName, setGroupName] = useState(group.name)
  const [description, setDescription] = useState(group.description)
  const [currency, setCurrency] = useState("USD")
  const [warrantyAlerts, setWarrantyAlerts] = useState(true)
  const [weeklyDigest, setWeeklyDigest] = useState(false)

  return (
    <div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">
      <div>
        <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Settings</h1>
        <p className="mt-1 text-muted-foreground">Manage your group and preferences.</p>
      </div>

      {/* Plan */}
      <div className="rounded-xl border border-border bg-card p-6 space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-9 items-center justify-center rounded-lg bg-accent/20 shrink-0">
              <Zap className="size-4 text-accent-foreground" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base font-semibold">Pro Plan</h2>
                <Badge variant="secondary" className="text-xs">Active</Badge>
              </div>
              <p className="text-sm text-muted-foreground mt-0.5">Owner: Alex Johnson</p>
            </div>
          </div>
          <Button variant="outline" size="sm" onClick={() => onNavigate?.("plans")}>Upgrade</Button>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-lg border border-border bg-muted/30 px-3 py-2.5">
            <div className="flex items-center gap-1.5 mb-1">
              <Package className="size-3.5 text-muted-foreground" />
              <p className="text-xs text-muted-foreground font-medium">Items</p>
            </div>
            <p className="text-sm font-semibold">9 <span className="text-muted-foreground font-normal">/ 500</span></p>
          </div>
          <div className="rounded-lg border border-border bg-muted/30 px-3 py-2.5">
            <div className="flex items-center gap-1.5 mb-1">
              <MapPin className="size-3.5 text-muted-foreground" />
              <p className="text-xs text-muted-foreground font-medium">Locations</p>
            </div>
            <p className="text-sm font-semibold">2 <span className="text-muted-foreground font-normal">/ 20</span></p>
          </div>
          <div className="rounded-lg border border-border bg-muted/30 px-3 py-2.5">
            <div className="flex items-center gap-1.5 mb-1">
              <FileText className="size-3.5 text-muted-foreground" />
              <p className="text-xs text-muted-foreground font-medium">File storage</p>
            </div>
            <p className="text-sm font-semibold">1.2 GB <span className="text-muted-foreground font-normal">/ 10 GB</span></p>
          </div>
        </div>

        <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 px-4 py-3">
          <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
          <p className="text-sm text-muted-foreground leading-relaxed">
            Group capabilities are tied to the owner's subscription. If the owner's plan is downgraded, some features will become unavailable for all members.
          </p>
        </div>
      </div>

      {/* Group */}
      <div className="rounded-xl border border-border bg-card p-6 space-y-5">
        <div>
          <h2 className="text-base font-semibold">Group</h2>
          <p className="text-sm text-muted-foreground mt-0.5">Settings for the active location group.</p>
        </div>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Group name</label>
            <Input
              value={groupName}
              onChange={(e) => setGroupName(e.target.value)}
              placeholder="Enter group name"
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-medium">Description</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Brief description of this group"
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-medium">Default currency</label>
            <CurrencyCombobox value={currency} onValueChange={setCurrency} />
          </div>
        </div>

        <div className="pt-2">
          <Button size="sm">Save changes</Button>
        </div>
      </div>

      {/* Notifications */}
      <div className="rounded-xl border border-border bg-card p-6 space-y-4">
        <h2 className="text-base font-semibold">Notifications</h2>

        <div className="divide-y divide-border">
          <div className="flex items-center justify-between py-3.5">
            <p className="text-sm font-medium">Warranty expiring alerts</p>
            <Switch checked={warrantyAlerts} onCheckedChange={setWarrantyAlerts} />
          </div>
          <div className="flex items-center justify-between py-3.5">
            <p className="text-sm font-medium">Weekly digest email</p>
            <Switch checked={weeklyDigest} onCheckedChange={setWeeklyDigest} />
          </div>
        </div>
      </div>

      {/* Data */}
      <div className="rounded-xl border border-border bg-card p-6 space-y-4">
        <h2 className="text-base font-semibold">Data</h2>

        <div className="divide-y divide-border">
          <button
            onClick={() => onNavigate?.("members")}
            className="flex w-full items-center gap-3 py-3.5 text-left hover:text-foreground transition-colors"
          >
            <Users className="size-4 text-muted-foreground shrink-0" />
            <span className="text-sm font-medium flex-1">Members</span>
            <ChevronRight className="size-4 text-muted-foreground" />
          </button>
          <button
            onClick={() => onNavigate?.("backup")}
            className="flex w-full items-center gap-3 py-3.5 text-left hover:text-foreground transition-colors"
          >
            <HardDriveDownload className="size-4 text-muted-foreground shrink-0" />
            <span className="text-sm font-medium flex-1">Backup &amp; Restore</span>
            <ChevronRight className="size-4 text-muted-foreground" />
          </button>
        </div>
      </div>

      {/* Danger zone */}
      <div className="rounded-xl border border-destructive/40 bg-card p-6 space-y-3">
        <h2 className="text-base font-semibold text-destructive">Danger zone</h2>
        <p className="text-sm text-muted-foreground">
          Deleting this group will permanently remove all its locations, items, and files. This action cannot be undone.
        </p>
        <Button
          variant="destructive"
          size="sm"
          className="gap-1.5"
        >
          <Trash2 className="size-3.5" />
          Delete this group
        </Button>
      </div>
    </div>
  )
}
