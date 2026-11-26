import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Library } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { CUSTOM_FIELD_LIMIT } from "@/features/customFields/api"
import { removalOf, type ItemSetView, type LocationGroup } from "@/features/group/api"
import {
  useAddCollectionSet,
  useCollectionSet,
  useRemoveCollectionSet,
} from "@/features/group/hooks"
import { ITEM_TYPE_LIMIT } from "@/features/itemTypes/api"
import { ITEM_TYPE_FALLBACK_ICON, ITEM_TYPE_ICON_LOOKUP } from "@/features/itemTypes/icons"
import { useAppToast } from "@/hooks/useAppToast"
import { formatList } from "@/lib/intl"
import { getServerErrorCode } from "@/lib/server-error"
import { cn } from "@/lib/utils"

import { CollectionSetRemoveDialog } from "./CollectionSetRemoveDialog"

// CollectionSetCard adds the Collection set to a group and removes it
// (#1607): item types for collections and the custom fields collectors
// track, shown on each of those types. Adding again creates only what is
// missing, so it also finishes an add that failed halfway. Removing asks
// first and lists what goes and what stays, from a status fetched again
// when the confirmation opens; the remove deletes only what it listed.
//
// Members without the admin role see the same card without the buttons.
interface CollectionSetCardProps {
  group: LocationGroup
  isAdmin: boolean
  className?: string
}

type SetState = "none" | "partial" | "complete"

type RowState = "missing" | "added" | "own" | "taken"

function setState(view: ItemSetView): SetState {
  const rows = [...(view.item_types ?? []), ...(view.custom_fields ?? [])]
  if (rows.every((r) => r.state === "missing")) return "none"
  return view.complete ? "complete" : "partial"
}

export function CollectionSetCard({ group, isAdmin, className }: CollectionSetCardProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const groupId = group.id ?? ""
  const groupSlug = group.slug ?? ""
  const query = useCollectionSet(groupId || undefined)
  const add = useAddCollectionSet()
  const remove = useRemoveCollectionSet()
  const [confirming, setConfirming] = useState(false)
  // When the confirmation opened: its lists wait for a status newer than
  // that.
  const [openedAt, setOpenedAt] = useState(0)
  // The status the confirmation was confirmed on. The dialog keeps showing
  // it while it closes, as the answer replaces the cached status first.
  const [confirmed, setConfirmed] = useState<ItemSetView | null>(null)

  const view = query.data
  const state = view ? setState(view) : "none"
  const pending = add.isPending || remove.isPending
  const hasSetRows =
    !!view &&
    [...(view.item_types ?? []), ...(view.custom_fields ?? [])].some((r) => r.state === "added")
  // A field of the set the group has that some of the set's types do not
  // offer: the set is partly added although every row is there.
  const linksMissing = (view?.custom_fields ?? []).some((r) => (r.missing_templates ?? 0) > 0)
  const checkFailed = query.isError && query.errorUpdatedAt >= openedAt && !query.isFetching
  const checking = !checkFailed && (query.isFetching || query.dataUpdatedAt < openedAt)

  function openConfirmation() {
    setConfirmed(null)
    setOpenedAt(Date.now())
    setConfirming(true)
    void query.refetch()
  }

  function onAdd() {
    add.mutate(
      { groupId, groupSlug },
      {
        onSuccess: ({ meta }) =>
          toast.success(
            t("groups:settings.collectionSet.added", {
              types: meta?.created_item_types ?? 0,
              fields: meta?.created_custom_fields ?? 0,
            })
          ),
        onError: (err) => toast.error(addErrorMessage(t, getServerErrorCode(err))),
      }
    )
  }

  function onRemove() {
    if (!view) return
    setConfirmed(view)
    remove.mutate(
      { groupId, groupSlug, removal: removalOf(view) },
      {
        onSuccess: ({ meta }) => {
          setConfirming(false)
          const deleted = [
            ...(meta?.deleted_item_types ?? []),
            ...(meta?.deleted_custom_fields ?? []),
          ]
          const kept = [...(meta?.kept_item_types ?? []), ...(meta?.kept_custom_fields ?? [])]
          if (deleted.length > 0) {
            toast.success(
              t("groups:settings.collectionSet.removed", { names: formatList(deleted) })
            )
          }
          if (kept.length > 0) {
            toast.warning(
              t("groups:settings.collectionSet.removedKept", {
                names: formatList(kept),
                count: kept.length,
              })
            )
          }
          if (deleted.length === 0 && kept.length === 0) {
            toast.info(t("groups:settings.collectionSet.removedNothing"))
          }
        },
        onError: () => {
          setConfirmed(null)
          toast.error(t("groups:settings.collectionSet.removeError"))
        },
      }
    )
  }

  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="collection-set-card"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3 min-w-0">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Library className="size-4 text-primary" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h2 className="text-base font-semibold">{t("groups:settings.collectionSet.title")}</h2>
            <p className="text-sm text-muted-foreground mt-0.5">
              {t("groups:settings.collectionSet.subtitle")}
            </p>
          </div>
        </div>
        {view ? <StateBadge state={state} /> : null}
      </div>

      {query.isPending ? (
        <p className="text-sm text-muted-foreground" data-testid="collection-set-loading">
          {t("common:loading")}
        </p>
      ) : !view ? (
        <p className="text-sm text-destructive" data-testid="collection-set-load-error">
          {t("groups:settings.collectionSet.loadError")}
        </p>
      ) : (
        <>
          <div className="space-y-3">
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                {t("groups:settings.collectionSet.itemTypes")}
              </p>
              <ul className="mt-2 flex flex-wrap gap-1.5" data-testid="collection-set-types">
                {(view.item_types ?? []).map((row) => {
                  const Icon =
                    (row.icon ? ITEM_TYPE_ICON_LOOKUP[row.icon] : undefined) ??
                    ITEM_TYPE_FALLBACK_ICON
                  const label = rowStateLabel(t, row.state as RowState, "type")
                  return (
                    <li key={row.key}>
                      <Badge
                        variant="outline"
                        className={cn("gap-1", absentRow(row.state) && absentClass)}
                        data-state={row.state}
                        title={label}
                        data-testid={`collection-set-type-${row.key}`}
                      >
                        <Icon aria-hidden="true" />
                        {row.name}
                        <span className="sr-only">, {label}</span>
                      </Badge>
                    </li>
                  )
                })}
              </ul>
            </div>
            <div>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                {t("groups:settings.collectionSet.customFields")}
              </p>
              <ul className="mt-2 flex flex-wrap gap-1.5" data-testid="collection-set-fields">
                {(view.custom_fields ?? []).map((row) => {
                  const missingLinks = row.missing_templates ?? 0
                  const label =
                    missingLinks > 0
                      ? `${rowStateLabel(t, row.state as RowState, "field")}; ${t(
                          "groups:settings.collectionSet.rowState.notOnTypes",
                          { count: missingLinks }
                        )}`
                      : rowStateLabel(t, row.state as RowState, "field")
                  return (
                    <li key={row.key}>
                      <Badge
                        variant="outline"
                        className={cn(absentRow(row.state) && absentClass)}
                        data-state={row.state}
                        title={label}
                        data-testid={`collection-set-field-${row.key}`}
                      >
                        {row.name}
                        <span className="sr-only">, {label}</span>
                      </Badge>
                    </li>
                  )
                })}
              </ul>
            </div>
            <p className="text-xs text-muted-foreground">
              {t("groups:settings.collectionSet.note")}
            </p>
            {linksMissing ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="collection-set-links-missing"
              >
                {t("groups:settings.collectionSet.linksMissing")}
              </p>
            ) : null}
          </div>

          {isAdmin ? (
            <div className="flex flex-wrap justify-end gap-2">
              {hasSetRows ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={openConfirmation}
                  disabled={pending || !groupId}
                  data-testid="collection-set-remove"
                >
                  {t("groups:settings.collectionSet.remove")}
                </Button>
              ) : null}
              {state !== "complete" ? (
                <Button
                  type="button"
                  onClick={onAdd}
                  disabled={pending || !groupId}
                  data-testid="collection-set-add"
                >
                  {add.isPending
                    ? t("groups:settings.collectionSet.adding")
                    : state === "partial"
                      ? t("groups:settings.collectionSet.addMissing")
                      : t("groups:settings.collectionSet.add")}
                </Button>
              ) : null}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground" data-testid="collection-set-admin-only">
              {t("groups:settings.collectionSet.adminOnly")}
            </p>
          )}

          <CollectionSetRemoveDialog
            open={confirming}
            view={confirmed ?? view}
            checking={confirmed ? false : checking}
            checkFailed={confirmed ? false : checkFailed}
            pending={remove.isPending}
            onConfirm={onRemove}
            onClose={() => setConfirming(false)}
          />
        </>
      )}
    </div>
  )
}

function StateBadge({ state }: { state: SetState }) {
  const { t } = useTranslation()
  switch (state) {
    case "complete":
      return (
        <Badge variant="secondary" data-testid="collection-set-state">
          {t("groups:settings.collectionSet.state.complete")}
        </Badge>
      )
    case "partial":
      return (
        <Badge variant="outline" data-testid="collection-set-state">
          {t("groups:settings.collectionSet.state.partial")}
        </Badge>
      )
    case "none":
      return (
        <Badge
          variant="outline"
          className="text-muted-foreground"
          data-testid="collection-set-state"
        >
          {t("groups:settings.collectionSet.state.none")}
        </Badge>
      )
  }
}

type TFn = ReturnType<typeof useTranslation>["t"]

// A row the group does not have gets a dashed, muted badge, so the state
// does not rest on color alone; the state is also spelled out for screen
// readers and on hover.
const absentClass = "border-dashed text-muted-foreground"

function absentRow(state: string | undefined): boolean {
  return state === "missing" || state === "taken"
}

function rowStateLabel(t: TFn, state: RowState, kind: "type" | "field"): string {
  switch (state) {
    case "added":
      return t("groups:settings.collectionSet.rowState.added")
    case "own":
      return t("groups:settings.collectionSet.rowState.own")
    case "taken":
      return kind === "type"
        ? t("groups:settings.collectionSet.rowState.takenType")
        : t("groups:settings.collectionSet.rowState.takenField")
    case "missing":
    default:
      return t("groups:settings.collectionSet.rowState.missing")
  }
}

function addErrorMessage(t: TFn, code: string | null): string {
  switch (code) {
    case ITEM_TYPE_LIMIT:
      return t("groups:settings.collectionSet.addErrorTypeLimit")
    case CUSTOM_FIELD_LIMIT:
      return t("groups:settings.collectionSet.addErrorFieldLimit")
    case "group.not_active":
      return t("groups:settings.collectionSet.addErrorNotActive")
    default:
      return t("groups:settings.collectionSet.addError")
  }
}
