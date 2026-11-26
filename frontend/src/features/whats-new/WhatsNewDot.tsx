import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

// The unread marker of the What's new entry and of the menus that lead to
// it (#1386): a small primary dot, with its meaning for screen readers.
// Where the dot has to sit outside the link it marks (the sidebar badge
// slot), render `WhatsNewUnreadLabel` inside the link instead and pass
// `labelled={false}` here.
export function WhatsNewDot({
  className,
  testId,
  labelled = true,
}: {
  className?: string
  testId?: string
  labelled?: boolean
}) {
  return (
    <>
      <span
        aria-hidden="true"
        data-testid={testId}
        className={cn("inline-block size-2 shrink-0 rounded-full bg-primary", className)}
      />
      {labelled ? <WhatsNewUnreadLabel /> : null}
    </>
  )
}

export function WhatsNewUnreadLabel() {
  const { t } = useTranslation()
  return <span className="sr-only">{`, ${t("settings:whatsNew.unread")}`}</span>
}
