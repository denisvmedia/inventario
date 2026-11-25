import { FileInput } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { DocumentSource as Source } from "@/features/intake/documents"
import { cn } from "@/lib/utils"

interface DocumentSourceProps {
  source: Source
  // "text" is the muted meta line of an inbox row; "value" is the value of
  // a detail row, without the icon the row already has.
  variant?: "text" | "value"
  className?: string
  testId?: string
}

// "From Paperless-ngx": the other system a draft or an item came from
// (#2809), linking to the document there when its address is known. The
// link opens in a new tab, since the other system is another app.
export function DocumentSource({
  source,
  variant = "text",
  className,
  testId,
}: DocumentSourceProps) {
  const { t } = useTranslation(["commodities"])
  const label =
    variant === "text" ? t("commodities:delivered.from", { name: source.name }) : source.name
  return (
    <span className={cn("inline-flex min-w-0 items-center gap-1", className)} data-testid={testId}>
      {variant === "text" ? <FileInput className="size-3 shrink-0" aria-hidden="true" /> : null}
      {source.url ? (
        <a
          href={source.url}
          target="_blank"
          rel="noreferrer noopener"
          className={cn("truncate hover:underline", variant === "value" && "text-primary")}
          data-testid={testId ? `${testId}-link` : undefined}
        >
          {label} <span className="sr-only">({t("commodities:delivered.newTab")})</span>
        </a>
      ) : (
        <span className="truncate">{label}</span>
      )}
    </span>
  )
}
