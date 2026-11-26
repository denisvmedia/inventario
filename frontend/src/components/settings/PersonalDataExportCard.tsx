import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import { Download, Loader2 } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DATA_EXPORT_IMPERSONATION_FORBIDDEN,
  DATA_EXPORT_NOT_FOUND,
  DATA_EXPORT_NOT_READY,
  DATA_EXPORT_RATE_LIMITED,
  isInProgress,
  type PersonalDataExport,
  type PersonalDataExportsList,
  type PersonalDataExportStatus,
} from "@/features/data-exports/api"
import {
  useDownloadPersonalDataExport,
  usePersonalDataExports,
  useRequestPersonalDataExport,
} from "@/features/data-exports/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { formatBytes, formatDate, formatDateTime } from "@/lib/intl"
import { getServerErrorCode, getServerErrorMeta } from "@/lib/server-error"

// PERSONAL_DATA_EXPORT_ANCHOR is the id of the section, so the delete-account
// dialog can send the user to it.
export const PERSONAL_DATA_EXPORT_ANCHOR = "download-my-data"

// The server keeps a finished copy this long; the list response says so too.
const DEFAULT_RETENTION_DAYS = 7

const STATUS_VARIANT: Record<PersonalDataExportStatus, "secondary" | "outline" | "destructive"> = {
  pending: "outline",
  in_progress: "outline",
  completed: "secondary",
  failed: "destructive",
}

// setTimeout holds a 32-bit number of milliseconds.
const MAX_TIMEOUT_MS = 2 ** 31 - 1

function requestedAtText(e: PersonalDataExport): string {
  return e.created_at
    ? formatDateTime(e.created_at, { dateStyle: "medium", timeStyle: "short" })
    : ""
}

// useHasPassed reports whether the time `at` has come, and re-renders when it
// does. Without `at` it is false.
function useHasPassed(at: string | undefined): boolean {
  const [passed, setPassed] = useState<string | undefined>(undefined)
  useEffect(() => {
    if (!at) return
    const delay = Date.parse(at) - Date.now()
    if (Number.isNaN(delay) || delay > MAX_TIMEOUT_MS) return
    const id = window.setTimeout(() => setPassed(at), Math.max(0, delay))
    return () => window.clearTimeout(id)
  }, [at])
  return !!at && passed === at
}

// settledMessage is what a screen reader hears when a copy that was being
// prepared becomes ready or fails between two loads of the list.
function settledMessage(
  before: PersonalDataExportsList | undefined,
  after: PersonalDataExportsList | undefined,
  t: TFunction
): string {
  if (!before || !after) return ""
  const was = new Map((before.exports ?? []).map((e) => [e.id, e.status]))
  let message = ""
  for (const e of after.exports ?? []) {
    if (!isInProgress(was.get(e.id)) || isInProgress(e.status)) continue
    if (e.status === "completed") {
      message = t("settings:account.dataExport.announceReady", { date: requestedAtText(e) })
    } else if (e.status === "failed") {
      message = t("settings:account.dataExport.announceFailed", { date: requestedAtText(e) })
    }
  }
  return message
}

// PersonalDataExportCard is the "Download my data" part of Account settings
// (#2177, GDPR Art. 20). It requests a copy of the user's own data, which the
// export worker builds in the background, and lists the copies until they
// expire. The request button stays disabled until the daily limit allows the
// next one.
export function PersonalDataExportCard() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const listQuery = usePersonalDataExports()
  const requestMutation = useRequestPersonalDataExport()

  const exports = listQuery.data?.exports ?? []
  const retentionDays = listQuery.data?.retention_days ?? DEFAULT_RETENTION_DAYS
  const nextRequestAt = listQuery.data?.next_request_at
  // The server sends next_request_at only while the daily limit holds. When
  // that time comes the button is enabled and the list loaded again.
  const nextRequestPassed = useHasPassed(nextRequestAt)
  const limited = !!nextRequestAt && !nextRequestPassed
  const { refetch } = listQuery
  useEffect(() => {
    if (nextRequestPassed) void refetch()
  }, [nextRequestPassed, refetch])

  // A copy finishing while the user waits is announced, not only shown.
  const [seenData, setSeenData] = useState(listQuery.data)
  const [announcement, setAnnouncement] = useState("")
  if (listQuery.data !== seenData) {
    setSeenData(listQuery.data)
    const message = settledMessage(seenData, listQuery.data, t)
    if (message) setAnnouncement(message)
  }

  async function handleRequest() {
    try {
      await requestMutation.mutateAsync()
      toast.success(t("settings:account.dataExport.requested"))
    } catch (err) {
      const code = getServerErrorCode(err)
      if (code === DATA_EXPORT_RATE_LIMITED) {
        const retryAt = getServerErrorMeta(err)?.retry_at
        toast.error(
          t("settings:account.dataExport.rateLimited", {
            date: retryAt
              ? formatDateTime(retryAt, { dateStyle: "medium", timeStyle: "short" })
              : "",
          })
        )
        return
      }
      if (code === DATA_EXPORT_IMPERSONATION_FORBIDDEN) {
        toast.error(t("settings:account.dataExport.impersonation"))
        return
      }
      toast.error(t("settings:account.dataExport.requestFailed"))
    }
  }

  return (
    <section
      id={PERSONAL_DATA_EXPORT_ANCHOR}
      tabIndex={-1}
      className="scroll-mt-20 rounded-xl border border-border p-4 space-y-3 outline-none"
      aria-labelledby="personal-data-export-title"
      aria-busy={listQuery.isLoading}
      data-testid="personal-data-export"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
        <div className="min-w-0">
          <h3 id="personal-data-export-title" className="text-sm font-medium">
            {t("settings:account.dataExport.title")}
          </h3>
          <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
            {t("settings:account.dataExport.description", { count: retentionDays })}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="shrink-0 gap-1.5 self-start"
          onClick={handleRequest}
          disabled={listQuery.isLoading || limited || requestMutation.isPending}
          data-testid="personal-data-export-request"
        >
          {requestMutation.isPending ? (
            <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
          ) : (
            <Download className="size-3.5" aria-hidden="true" />
          )}
          {requestMutation.isPending
            ? t("settings:account.dataExport.requesting")
            : t("settings:account.dataExport.request")}
        </Button>
      </div>

      {limited ? (
        <p className="text-xs text-muted-foreground" data-testid="personal-data-export-next">
          {t("settings:account.dataExport.nextRequestAt", {
            date: formatDateTime(nextRequestAt, { dateStyle: "medium", timeStyle: "short" }),
          })}
        </p>
      ) : null}

      <p
        className="sr-only"
        role="status"
        aria-live="polite"
        data-testid="personal-data-export-announcement"
      >
        {announcement}
      </p>

      {listQuery.isError ? (
        <p className="text-xs text-destructive" role="alert">
          {t("settings:account.dataExport.loadFailed")}
        </p>
      ) : null}

      {exports.length > 0 ? (
        <ul
          className="rounded-xl border border-border divide-y divide-border"
          data-testid="personal-data-export-list"
        >
          {exports.map((e) => (
            <PersonalDataExportRow key={e.id} item={e} />
          ))}
        </ul>
      ) : null}
    </section>
  )
}

function PersonalDataExportRow({ item }: { item: PersonalDataExport }) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const downloadMutation = useDownloadPersonalDataExport()
  const status: PersonalDataExportStatus = item.status ?? "pending"
  const requestedAt = requestedAtText(item)

  async function handleDownload() {
    if (!item.id) return
    try {
      await downloadMutation.mutateAsync(item.id)
    } catch (err) {
      // The hook reloads the list on any failure; the message says why.
      switch (getServerErrorCode(err)) {
        case DATA_EXPORT_IMPERSONATION_FORBIDDEN:
          toast.error(t("settings:account.dataExport.impersonation"))
          return
        case DATA_EXPORT_NOT_FOUND:
          toast.error(t("settings:account.dataExport.downloadGone"))
          return
        case DATA_EXPORT_NOT_READY:
          toast.error(t("settings:account.dataExport.downloadNotReady"))
          return
      }
      toast.error(t("settings:account.dataExport.downloadFailed"))
    }
  }

  return (
    <li
      className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between sm:gap-4"
      data-testid="personal-data-export-row"
      data-status={status}
    >
      <div className="min-w-0 space-y-0.5">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium">
            {t("settings:account.dataExport.requestedAt", { date: requestedAt })}
          </p>
          <Badge variant={STATUS_VARIANT[status]} className="gap-1 text-xs">
            {isInProgress(status) ? (
              <Loader2 className="size-3 animate-spin" aria-hidden="true" />
            ) : null}
            {t(`settings:account.dataExport.status.${status}`)}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground">
          {status === "completed"
            ? t("settings:account.dataExport.ready", {
                size: formatBytes(item.size_bytes ?? 0),
                date: item.expires_at ? formatDate(item.expires_at, { style: "medium" }) : "",
              })
            : status === "failed"
              ? t("settings:account.dataExport.failed")
              : t("settings:account.dataExport.preparing")}
        </p>
      </div>
      {status === "completed" ? (
        <Button
          variant="outline"
          size="sm"
          className="shrink-0 gap-1.5 self-start sm:self-auto"
          onClick={handleDownload}
          disabled={downloadMutation.isPending}
          aria-label={t("settings:account.dataExport.downloadLabel", { date: requestedAt })}
          data-testid="personal-data-export-download"
        >
          <Download className="size-3.5" aria-hidden="true" />
          {t("settings:account.dataExport.download")}
        </Button>
      ) : null}
    </li>
  )
}
