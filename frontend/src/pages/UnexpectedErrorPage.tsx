import { AlertTriangle, RotateCw } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { ErrorInfo } from "react"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useSystemInfo } from "@/features/system/hooks"
import { isUiDebugOverrideEnabled } from "@/lib/ui-debug"

interface UnexpectedErrorPageProps {
  error: Error
  errorInfo?: ErrorInfo | null
  onReset: () => void
}

// Full-screen error surface rendered by RootErrorBoundary when a
// render-time exception escapes a route. Stays inside our design
// language (status-* / destructive tokens, no raw colors) and shows a
// details panel with stack + component stack so the browser console
// isn't the only place to see what blew up. The panel is OFF for
// production users by default — it shows only when (#1965):
//   - this is a local vite dev build (`import.meta.env.DEV`), OR
//   - the backend reports debug mode on (`system.debug`, driven by
//     INVENTARIO_DEBUG_UI — preview / demo deploys set it), OR
//   - the viewer deliberately opted in for their own browser via
//     `?debug=1` (a low-risk, per-browser escape hatch that works in any
//     env — see lib/ui-debug for the security rationale).
// `useSystemInfo` is already primed on authed pages (CommitBadge fetches
// it), so reading it here is normally a cache hit.
export function UnexpectedErrorPage({ error, errorInfo, onReset }: UnexpectedErrorPageProps) {
  const { t } = useTranslation()
  const { data: system } = useSystemInfo()
  const showDetails = import.meta.env.DEV || system?.debug === true || isUiDebugOverrideEnabled()
  return (
    <>
      <RouteTitle title={t("errors:unexpected.documentTitle")} />
      <div className="flex min-h-screen items-start justify-center px-6 py-16">
        <div
          className="flex w-full max-w-2xl flex-col items-center gap-6 text-center"
          data-testid="page-unexpected-error"
        >
          <div className="flex size-14 items-center justify-center rounded-2xl bg-destructive/10">
            <AlertTriangle className="size-7 text-destructive" aria-hidden="true" />
          </div>
          <div className="flex flex-col gap-2">
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
              {t("errors:unexpected.heading")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("errors:unexpected.description")}</p>
          </div>
          <div className="flex flex-wrap items-center justify-center gap-2">
            <Button onClick={() => window.location.reload()} className="gap-1.5">
              <RotateCw className="size-4" aria-hidden="true" />
              {t("errors:unexpected.reload")}
            </Button>
            <Button variant="outline" onClick={onReset}>
              {t("errors:unexpected.retry")}
            </Button>
          </div>
          {showDetails ? (
            <div
              className="w-full overflow-hidden rounded-xl border border-destructive/20 bg-destructive/5 text-left"
              data-testid="unexpected-error-details"
            >
              <div className="border-b border-destructive/20 px-4 py-2.5">
                <p className="text-xs font-semibold uppercase tracking-widest text-destructive">
                  {t("errors:unexpected.devDetails")}
                </p>
              </div>
              <div className="space-y-3 p-4">
                <p className="font-mono text-sm font-semibold text-destructive">{error.message}</p>
                {error.stack ? (
                  <pre className="whitespace-pre-wrap break-words font-mono text-xs text-foreground/80">
                    {error.stack}
                  </pre>
                ) : null}
                {errorInfo?.componentStack ? (
                  <pre className="whitespace-pre-wrap break-words font-mono text-xs text-muted-foreground">
                    {errorInfo.componentStack}
                  </pre>
                ) : null}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </>
  )
}
