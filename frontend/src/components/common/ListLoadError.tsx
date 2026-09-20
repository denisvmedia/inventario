import { useTranslation } from "react-i18next"
import { AlertCircle } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"

interface ListLoadErrorProps {
  // Suffix for the test id, so a page's error state is addressable the way its
  // loading and empty states already are.
  testId: string
  onRetry?: () => void
}

// Shown when a list query fails. Without it a failed load falls through to the
// empty state, and "No items" after a dropped connection reads as data loss
// rather than a network problem (#2098).
export function ListLoadError({ testId, onRetry }: ListLoadErrorProps) {
  const { t } = useTranslation()
  return (
    <Alert variant="destructive" data-testid={testId}>
      <AlertCircle className="size-4" aria-hidden="true" />
      <AlertTitle>{t("common:listError.title")}</AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-3">
        <span>{t("common:listError.description")}</span>
        {onRetry ? (
          <Button variant="outline" size="sm" onClick={onRetry}>
            {t("common:serverError.retry")}
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  )
}
