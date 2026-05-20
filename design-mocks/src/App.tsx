import { useState, useEffect } from "react"
import { OnboardingTour, RestartTourButton, TOUR_STEPS } from "@/components/OnboardingTour"
import { useOnboarding } from "@/hooks/use-onboarding"
import { SidebarProvider, SidebarTrigger, SidebarInset } from "@/components/ui/sidebar"
import { AppSidebar } from "@/components/AppSidebar"
import { ItemDetail } from "@/components/ItemDetail"
import { AddItemDialog } from "@/components/AddItemDialog"
import { DashboardView } from "@/views/DashboardView"
import { ItemsView } from "@/views/ItemsView"
import { WarrantiesView } from "@/views/WarrantiesView"
import { SettingsView } from "@/views/SettingsView"
import { GroupSettingsView } from "@/views/GroupSettingsView"
import { LocationPickerView } from "@/views/LocationPickerView"
import { FileBrowserView } from "@/views/FileBrowserView"
import { MembersView } from "@/views/MembersView"
import { BackupView } from "@/views/BackupView"
import { UserProfileView } from "@/views/UserProfileView"
import { EditProfileView } from "@/views/EditProfileView"
import { ImageViewerView } from "@/views/ImageViewerView"
import { PdfViewerView } from "@/views/PdfViewerView"
import { InsuranceReportView } from "@/views/InsuranceReportView"
import { UIShowcaseView } from "@/views/UIShowcaseView"
import { TagsView } from "@/views/TagsView"
import { PlansView } from "@/views/PlansView"
import { AuthView } from "@/views/AuthView"
import { TenantsView } from "@/views/admin/TenantsView"
import { TenantDetailView } from "@/views/admin/TenantDetailView"
import { UserDetailView } from "@/views/admin/UserDetailView"
import { GroupsView } from "@/views/admin/GroupsView"
import { GroupDetailView } from "@/views/admin/GroupDetailView"
import { ImpersonationBannerMock } from "@/views/admin/ImpersonationBannerMock"
import { adminUserById } from "@/data/mock"
import {
  NotFoundView,
  NoLocationGroupView,
  NoGroupOnboardingView,
  NoLocationView,
  NoAreaView,
  MaintenanceView,
} from "@/views/EmptyStatesView"
import { InviteBanner } from "@/components/InviteBanner"
import { ModeToggle } from "@/components/mode-toggle"
import { CommandPalette, useCommandPalette } from "@/components/CommandPalette"
import { KeyboardShortcutsDialog } from "@/components/KeyboardShortcutsDialog"
import { Separator } from "@/components/ui/separator"
import {
  Package,
  Tag,
  LayoutDashboard,
  ShieldCheck,
  Settings,
  FolderOpen,
  MapPin,
  User,
  Image,
  FileText,
  LogIn,
  Users,
  HardDriveDownload,
  SearchX,
  Layers,
  LayoutGrid,
  Wrench,
  Shield,
  Palette,
  Zap,
  Search,
  Building2,
  UserCog,
} from "lucide-react"
import { Kbd, KbdGroup } from "@/components/ui/kbd"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

type View =
  | "dashboard"
  | "items"
  | "warranties"
  | "tags"
  | "settings"
  | "group-settings"
  | "locations"
  | "files"
  | "members"
  | "backup"
  | "profile"
  | "edit-profile"
  | "plans"
  | "image-viewer"
  | "pdf-viewer"
  | "insurance-report"
  | "ui-showcase"
  | "auth"
  | "state-404"
  | "state-no-group"
  | "state-no-location"
  | "state-no-area"
  | "state-maintenance"
  | "state-no-group-onboarding"
  | "admin-tenants"
  | "admin-tenant-detail"
  | "admin-user-detail"
  | "admin-groups"
  | "admin-group-detail"

const VIEW_TITLES: Record<View, string> = {
  dashboard: "Dashboard",
  items: "All Items",
  warranties: "Warranties",
  tags: "Tags",
  settings: "Preferences",
  "group-settings": "Group Settings",
  locations: "Locations",
  files: "Files",
  members: "Members",
  backup: "Backup & Restore",
  profile: "Profile",
  "edit-profile": "Edit Profile",
  plans: "Plans & Pricing",
  "image-viewer": "Image Viewer",
  "pdf-viewer": "PDF Viewer",
  "insurance-report": "Insurance Report",
  "ui-showcase": "UI Showcase",
  auth: "Sign In",
  "state-404": "404 Not Found",
  "state-no-group": "No Location Group",
  "state-no-location": "No Location",
  "state-no-area": "No Area",
  "state-maintenance": "Maintenance",
  "state-no-group-onboarding": "Get Started",
  "admin-tenants": "Tenants",
  "admin-tenant-detail": "Tenant Detail",
  "admin-user-detail": "User Detail",
  "admin-groups": "Groups",
  "admin-group-detail": "Group Detail",
}

const VIEW_ICONS: Record<View, React.ElementType> = {
  dashboard: LayoutDashboard,
  items: Package,
  warranties: ShieldCheck,
  tags: Tag,
  settings: Settings,
  "group-settings": Settings,
  locations: MapPin,
  files: FolderOpen,
  members: Users,
  backup: HardDriveDownload,
  profile: User,
  "edit-profile": User,
  plans: Zap,
  "image-viewer": Image,
  "pdf-viewer": FileText,
  "insurance-report": Shield,
  "ui-showcase": Palette,
  auth: LogIn,
  "state-404": SearchX,
  "state-no-group": Layers,
  "state-no-location": MapPin,
  "state-no-area": LayoutGrid,
  "state-maintenance": Wrench,
  "state-no-group-onboarding": Layers,
  "admin-tenants": Building2,
  "admin-tenant-detail": Building2,
  "admin-user-detail": UserCog,
  "admin-groups": Layers,
  "admin-group-detail": Layers,
}


export function App() {
  const [view, setView] = useState<View>("dashboard")
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null)
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const [activeGroupId, setActiveGroupId] = useState("g1")
  const [insuranceReportItemId, setInsuranceReportItemId] = useState<string | undefined>()
  const [insuranceReportLocationId, setInsuranceReportLocationId] = useState<string | undefined>()
  const [selectedTenantId, setSelectedTenantId] = useState<string | null>(null)
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null)
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null)
  const [impersonatingUserId, setImpersonatingUserId] = useState<string | null>(null)
  // Remember which admin view a detail page was opened from, so "Back"
  // returns to the originating context (tenant detail vs. a list view).
  const [userDetailBackTo, setUserDetailBackTo] = useState<View>("admin-tenants")
  const [groupDetailBackTo, setGroupDetailBackTo] = useState<View>("admin-groups")
  const onboarding = useOnboarding()
  const palette = useCommandPalette()
  const [shortcutsOpen, setShortcutsOpen] = useState(false)

  // Global keyboard shortcuts
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      // Cmd/Ctrl+/ → shortcuts dialog
      if (e.key === "/" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setShortcutsOpen((o) => !o)
        return
      }
      // Cmd/Ctrl+N → add item
      if (e.key === "n" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setAddDialogOpen(true)
        return
      }
      // G+x two-key nav (only when no input focused)
      const tag = (document.activeElement as HTMLElement)?.tagName?.toLowerCase()
      if (tag === "input" || tag === "textarea" || (document.activeElement as HTMLElement)?.isContentEditable) return
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [])

  const fullscreenViews: View[] = ["auth", "image-viewer", "pdf-viewer", "insurance-report"]
  const isFullscreen = fullscreenViews.includes(view)

  if (isFullscreen) {
    return (
      <>
        {view === "auth" && <AuthView onAuth={() => setView("dashboard")} />}
        {view === "image-viewer" && (
          <ImageViewerView onClose={() => setView("files")} />
        )}
        {view === "pdf-viewer" && (
          <PdfViewerView onClose={() => setView("files")} />
        )}
        {view === "insurance-report" && (
          <InsuranceReportView
            initialItemId={insuranceReportItemId}
            initialLocationId={insuranceReportLocationId}
            onBack={() => setView("items")}
          />
        )}
      </>
    )
  }

  const ViewIcon = VIEW_ICONS[view]
  const impersonatingUser = impersonatingUserId ? adminUserById(impersonatingUserId) : undefined

  return (
    <SidebarProvider>
      <AppSidebar
        activeView={view}
        onNavigate={(v) => setView(v as View)}
        onAddItem={() => setAddDialogOpen(true)}
        activeGroupId={activeGroupId}
        onGroupChange={setActiveGroupId}
      />
      <SidebarInset>
        {/* Impersonation banner — non-dismissible, sits above the topbar */}
        {impersonatingUser && (
          <ImpersonationBannerMock
            userName={impersonatingUser.name}
            userEmail={impersonatingUser.email}
            onEnd={() => setImpersonatingUserId(null)}
          />
        )}
        {/* Topbar */}
        <header className="flex h-12 items-center gap-2 border-b border-border px-4 sticky top-0 bg-background z-10" data-tour="welcome">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="h-4" />
          <div className="flex items-center gap-2 text-sm">
            <ViewIcon className="size-4 text-muted-foreground" />
            <span className="font-medium">{VIEW_TITLES[view]}</span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            {/* Demo navigation shortcuts */}
            <div className="hidden sm:flex items-center gap-1 border border-border rounded-lg p-1">
              {(["auth", "image-viewer", "pdf-viewer", "insurance-report", "ui-showcase", "state-no-group-onboarding"] as View[]).map((v) => {
                const Icon = VIEW_ICONS[v]
                return (
                  <button
                    key={v}
                    onClick={() => setView(v)}
                    className="flex items-center gap-1 rounded px-2 py-0.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
                    title={VIEW_TITLES[v]}
                  >
                    <Icon className="size-3.5" />
                    <span>{VIEW_TITLES[v]}</span>
                  </button>
                )
              })}
            </div>
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  onClick={() => palette.setOpen(true)}
                  className="hidden sm:flex items-center gap-2 h-8 px-3 rounded-md border border-border bg-background text-muted-foreground text-sm hover:bg-muted hover:text-foreground transition-colors"
                >
                  <Search className="size-3.5 shrink-0" />
                  <span className="text-xs">Search…</span>
                  <KbdGroup className="ml-1">
                    <Kbd className="text-[10px] h-4 min-w-4 px-1">⌘</Kbd>
                    <Kbd className="text-[10px] h-4 min-w-4 px-1">K</Kbd>
                  </KbdGroup>
                </button>
              </TooltipTrigger>
              <TooltipContent side="bottom">Open command palette</TooltipContent>
            </Tooltip>
            <RestartTourButton onRestart={onboarding.restart} />
            <ModeToggle />
          </div>
        </header>

        {/* Invite Banner */}
        <InviteBanner count={1} onViewInvites={() => setView("members")} />

        {/* Main content */}
        <main className="flex-1 overflow-y-auto">
          {view === "dashboard" && <DashboardView onItemClick={setSelectedItemId} onAddItem={() => setAddDialogOpen(true)} />}
          {view === "items" && <ItemsView onItemClick={setSelectedItemId} onAddItem={() => setAddDialogOpen(true)} />}
          {view === "warranties" && <WarrantiesView onItemClick={setSelectedItemId} />}
          {view === "tags" && <TagsView />}
          {view === "settings" && <SettingsView onNavigate={(v) => setView(v as View)} onOpenShortcuts={() => setShortcutsOpen(true)} />}
          {view === "group-settings" && (
            <GroupSettingsView
              activeGroupId={activeGroupId}
              onNavigate={(v) => setView(v as View)}
            />
          )}
          {view === "locations" && (
            <LocationPickerView
              activeGroupId={activeGroupId}
              onItemClick={setSelectedItemId}
            />
          )}
          {view === "files" && (
            <FileBrowserView
              onOpenFile={(file) => {
                if (file.mimeType === "application/pdf") setView("pdf-viewer")
                else if (file.mimeType.startsWith("image/")) setView("image-viewer")
              }}
            />
          )}
          {view === "members" && <MembersView activeGroupId={activeGroupId} />}
          {view === "backup" && <BackupView />}
          {view === "profile" && (
            <UserProfileView
              onItemClick={setSelectedItemId}
              onEditProfile={() => setView("edit-profile")}
              onUpgrade={() => setView("plans")}
            />
          )}
          {view === "edit-profile" && (
            <EditProfileView onBack={() => setView("profile")} />
          )}
          {view === "plans" && (
            <PlansView onBack={() => setView("group-settings")} />
          )}
          {view === "ui-showcase" && <UIShowcaseView />}
          {view === "state-404" && <NotFoundView onGoHome={() => setView("dashboard")} />}
          {view === "state-no-group" && <NoLocationGroupView />}
          {view === "state-no-group-onboarding" && (
            <NoGroupOnboardingView onCreateGroup={() => setView("members")} />
          )}
          {view === "state-no-location" && <NoLocationView />}
          {view === "state-no-area" && <NoAreaView />}
          {view === "state-maintenance" && <MaintenanceView />}

          {/* Admin views */}
          {view === "admin-tenants" && (
            <TenantsView
              onSelectTenant={(id) => {
                setSelectedTenantId(id)
                setView("admin-tenant-detail")
              }}
            />
          )}
          {view === "admin-tenant-detail" && selectedTenantId && (
            <TenantDetailView
              tenantId={selectedTenantId}
              onBack={() => setView("admin-tenants")}
              onSelectUser={(id) => {
                setSelectedUserId(id)
                setUserDetailBackTo("admin-tenant-detail")
                setView("admin-user-detail")
              }}
              onSelectGroup={(id) => {
                setSelectedGroupId(id)
                setGroupDetailBackTo("admin-tenant-detail")
                setView("admin-group-detail")
              }}
            />
          )}
          {view === "admin-user-detail" && selectedUserId && (
            <UserDetailView
              key={selectedUserId}
              userId={selectedUserId}
              onBack={() => setView(userDetailBackTo)}
              onSelectGroup={(id) => {
                setSelectedGroupId(id)
                setGroupDetailBackTo("admin-user-detail")
                setView("admin-group-detail")
              }}
              onImpersonate={(id) => setImpersonatingUserId(id)}
            />
          )}
          {view === "admin-groups" && (
            <GroupsView
              onSelectGroup={(id) => {
                setSelectedGroupId(id)
                setGroupDetailBackTo("admin-groups")
                setView("admin-group-detail")
              }}
            />
          )}
          {view === "admin-group-detail" && selectedGroupId && (
            <GroupDetailView
              key={selectedGroupId}
              groupId={selectedGroupId}
              onBack={() => setView(groupDetailBackTo)}
            />
          )}
        </main>
      </SidebarInset>

      <ItemDetail
        itemId={selectedItemId}
        onClose={() => setSelectedItemId(null)}
        onOpenInsuranceReport={(itemId) => {
          setInsuranceReportItemId(itemId)
          setInsuranceReportLocationId(undefined)
          setView("insurance-report")
        }}
      />
      <AddItemDialog open={addDialogOpen} onClose={() => setAddDialogOpen(false)} />

      {onboarding.active && (
        <OnboardingTour
          step={onboarding.step}
          totalSteps={TOUR_STEPS.length}
          onNext={onboarding.next}
          onPrev={onboarding.prev}
          onFinish={onboarding.finish}
          onSkip={onboarding.skip}
        />
      )}

      <CommandPalette
        open={palette.open}
        onOpenChange={palette.setOpen}
        onNavigate={(v) => setView(v as View)}
        onItemClick={setSelectedItemId}
      />
      <KeyboardShortcutsDialog
        open={shortcutsOpen}
        onOpenChange={setShortcutsOpen}
      />
    </SidebarProvider>
  )
}

export default App
