import { useState } from "react"
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
} from "lucide-react"

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
}


export function App() {
  const [view, setView] = useState<View>("dashboard")
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null)
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const [activeGroupId, setActiveGroupId] = useState("g1")
  const [insuranceReportItemId, setInsuranceReportItemId] = useState<string | undefined>()
  const [insuranceReportLocationId, setInsuranceReportLocationId] = useState<string | undefined>()
  const onboarding = useOnboarding()

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
          {view === "settings" && <SettingsView onNavigate={(v) => setView(v as View)} />}
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
    </SidebarProvider>
  )
}

export default App
