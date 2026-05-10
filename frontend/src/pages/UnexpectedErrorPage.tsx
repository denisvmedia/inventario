import { AlertTriangle, RotateCw } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { ErrorInfo } from "react"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"

interface UnexpectedErrorPageProps {
  error: Error
  errorInfo?: ErrorInfo | null
  onReset: () => void
}

// Full-screen error surface rendered by RootErrorBoundary when a
// render-time exception escapes a route. Stays inside our design
// language (status-* / destructive tokens, no raw colors) and shows
// a dev-only details panel with stack + component stack so the dev
// console isn't the only place to see what blew up.
export function UnexpectedErrorPage({ error, errorInfo, onReset }: UnexpectedErrorPageProps) {
  const { t } = useTranslation()
  const isDev = import.meta.env.DEV
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
            <p className="text-sm text-muted-foreground">
              {t("errors:unexpected.description")}
            </p>
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
          {isDev ? (
            <div className="w-full overflow-hidden rounded-xl border border-destructive/20 bg-destructive/5 text-left">
              <div className="border-b border-destructive/20 px-4 py-2.5">
                <p className="text-xs font-semibold uppercase tracking-widest text-destructive">
                  {t("errors:unexpected.devDetails")}
                </p>
              </div>
              <div className="space-y-3 p-4">
                <p className="font-mono text-sm font-semibold text-destructive">
                  {error.message}
                </p>
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
