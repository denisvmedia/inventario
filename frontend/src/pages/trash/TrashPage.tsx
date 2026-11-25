import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"
import { Trash2 } from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { TrashedFilesTab } from "@/components/trash/TrashedFilesTab"
import { TrashedItemsTab } from "@/components/trash/TrashedItemsTab"
import { Button } from "@/components/ui/button"
import { Page, PageHeader } from "@/components/ui/page"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  useEmptyTrash,
  useTrashPermissions,
  useTrashedCommodities,
  useTrashedFiles,
} from "@/features/trash/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"

type TrashTab = "items" | "files"

// The tabs read the first page with the same options, so these counts share
// the tabs' cached queries.
const FIRST_PAGE = { page: 1, perPage: 50 }

// TrashPage lists what members deleted in the group: items (with the files
// that went with them) and files deleted on their own. Viewers see it, users
// restore, admins and owners delete for good or empty it. The server purges
// entries once the retention runs out.
export function TrashPage() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const permissions = useTrashPermissions()
  const [searchParams, setSearchParams] = useSearchParams()
  const tab: TrashTab = searchParams.get("tab") === "files" ? "files" : "items"

  const items = useTrashedCommodities(FIRST_PAGE)
  const files = useTrashedFiles(FIRST_PAGE)
  const itemCount = items.data?.total ?? 0
  const fileCount = files.data?.total ?? 0
  const retentionDays = items.data?.retentionDays || files.data?.retentionDays || 0
  const emptyMutation = useEmptyTrash()

  function setTab(next: string) {
    const params = new URLSearchParams(searchParams)
    if (next === "files") params.set("tab", "files")
    else params.delete("tab")
    setSearchParams(params, { replace: true })
  }

  async function onEmpty() {
    const ok = await confirm({
      title: t("trash:confirm.emptyTitle"),
      description: t("trash:confirm.emptyBody"),
      confirmLabel: t("trash:actions.empty"),
      destructive: true,
    })
    if (!ok) return
    try {
      const result = await emptyMutation.mutateAsync()
      toast.success(t("trash:toast.emptied", { count: result.commodities + result.files }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Page width="wide" data-testid="page-trash">
      <RouteTitle title={t("trash:title")} />
      <PageHeader
        title={t("trash:title")}
        subtitle={
          retentionDays > 0
            ? t("trash:subtitle", { count: retentionDays })
            : t("trash:subtitleNoRetention")
        }
        subtitleClassName="max-w-prose"
        actions={
          permissions.canPurge && itemCount + fileCount > 0 ? (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => void onEmpty()}
              disabled={emptyMutation.isPending}
              data-testid="trash-empty"
            >
              <Trash2 className="size-4" aria-hidden="true" />
              {t("trash:actions.empty")}
            </Button>
          ) : null
        }
      />

      <Tabs value={tab} onValueChange={setTab} data-testid="trash-tabs">
        <TabsList variant="line" className="justify-start">
          <TabsTrigger value="items" data-testid="trash-tab-items">
            {t("trash:tabs.items", { n: itemCount })}
          </TabsTrigger>
          <TabsTrigger value="files" data-testid="trash-tab-files">
            {t("trash:tabs.files", { n: fileCount })}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="items" className="mt-4">
          <TrashedItemsTab permissions={permissions} />
        </TabsContent>
        <TabsContent value="files" className="mt-4">
          <TrashedFilesTab permissions={permissions} />
        </TabsContent>
      </Tabs>
    </Page>
  )
}
