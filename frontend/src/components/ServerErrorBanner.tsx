import { useTranslation } from "react-i18next"
import { AlertTriangle, RefreshCw, WifiOff } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { type ClassifiedServerError, isRetryableKind } from "@/lib/server-error"

interface ServerErrorBannerProps {
  // Classified error from `classifyServerError`. Pass `null` to render
  // nothing — callers can wire `{error ? <ServerErrorBanner … /> : null}`
  // or `<ServerErrorBanner error={error} />` either way works.
  error: ClassifiedServerError | null
  // Re-run the failing operation. Only shown for `network` / `unknown`
  // kinds (validation / conflict need user action — see `isRetryableKind`).
  // Omit the prop entirely to hide Retry even for retryable kinds (e.g.
  // when the calling form doesn't support an in-place retry).
  onRetry?: () => void
  // True while a retry is in flight — disables the Retry button so the
  // user can't stack requests. Caller drives this from its own
  // mutation/submitting state.
  isRetrying?: boolean
  // Stable hook for E2E + integration tests so they don't have to
  // text-match the headline copy.
  testId?: string
  // Optional override for the headline copy. Defaults to
  // `common:serverError.<kind>.title` and is rarely needed — surface-
  // specific copy can live in the message itself.
  titleOverride?: string
  // Optional className passthrough for wrapper layout (e.g. `mt-3`).
  className?: string
}

// Typed server-error banner — single visual contract across the app so
// users learn the affordances (Retry vs. fix-and-resubmit) once. Kind
// classification lives in `lib/server-error.ts`; this component is just
// a presentational consumer.
export function ServerErrorBanner({
  error,
  onRetry,
  isRetrying,
  testId,
  titleOverride,
  className,
}: ServerErrorBannerProps) {
  const { t } = useTranslation()
  if (!error) return null
  const { kind, message } = error
  const title = titleOverride ?? t(`common:serverError.${kind}.title`)
  const Icon = kind === "network" ? WifiOff : AlertTriangle
  const showRetry = !!onRetry && isRetryableKind(kind)
  return (
    <Alert variant="destructive" className={className} data-testid={testId} data-error-kind={kind}>
      <Icon className="size-4" aria-hidden="true" />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <p>{message}</p>
        {showRetry ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onRetry}
            disabled={isRetrying}
            className="mt-2"
            data-testid={testId ? `${testId}-retry` : undefined}
          >
            <RefreshCw className="size-3.5" aria-hidden="true" />
            {t("common:serverError.retry")}
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  )
}
