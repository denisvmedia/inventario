import type { ComponentProps, ReactNode } from "react"

import { cn } from "@/lib/utils"

/*
  Page + PageHeader — issue #1889.

  Single source of truth for top-level route layout. Every page under
  `src/pages/**` whose role is "top-level route" should compose with these
  primitives instead of reaching for ad-hoc `max-w-*` / `<h1>` markup. New
  pages picking a one-off width or rolling their own title are caught by the
  `src/pages/__tests__/page-layout-tokens.test.ts` convention guard, so the
  envelope stays consistent over time.

  Width tokens live in `index.css` (`--container-page-narrow` /
  `--container-page-wide`); the `width` prop selects between them or opts
  out of a max-width entirely.
*/

const WIDTH_CLASSES = {
  narrow: "max-w-page-narrow",
  wide: "max-w-page-wide",
  full: "",
} as const

export type PageWidth = keyof typeof WIDTH_CLASSES

type PageProps = ComponentProps<"div"> & {
  width?: PageWidth
}

export function Page({ width = "wide", className, children, ...props }: PageProps) {
  return (
    <div
      data-slot="page"
      data-page-width={width}
      className={cn("mx-auto flex w-full flex-col gap-6 p-6", WIDTH_CLASSES[width], className)}
      {...props}
    >
      {children}
    </div>
  )
}

type PageHeaderSize = "page" | "detail"

// Omit `title` from the native <header> attributes — the HTML `title`
// global attribute is a tooltip string and would collide with our ReactNode
// `title` slot otherwise.
type PageHeaderProps = Omit<ComponentProps<"header">, "title"> & {
  title: ReactNode
  subtitle?: ReactNode
  icon?: ReactNode
  actions?: ReactNode
  backLink?: ReactNode
  size?: PageHeaderSize
  /**
   * Extra classes appended to the rendered `<h1>` element. Reserved for
   * callers that need to tweak the canonical typography (e.g. a richer
   * status-badge cluster wrapped inside the title slot, or a tighter
   * tracking override). Does NOT replace the `<h1>` wrapper — the heading
   * always renders as an `<h1>` so the route's accessible name stays
   * stable.
   */
  titleClassName?: string
  /**
   * Extra classes appended to the rendered subtitle `<p>` (e.g.
   * `max-w-prose`, `text-sm`).
   */
  subtitleClassName?: string
}

const TITLE_BASE = "scroll-m-20 font-semibold tracking-tight flex items-center gap-2 min-w-0"

const TITLE_SIZE: Record<PageHeaderSize, string> = {
  page: "text-3xl",
  detail: "text-2xl",
}

export function PageHeader({
  title,
  subtitle,
  icon,
  actions,
  backLink,
  size = "page",
  className,
  titleClassName,
  subtitleClassName,
  ...props
}: PageHeaderProps) {
  return (
    <header
      data-slot="page-header"
      data-page-header-size={size}
      className={cn("flex flex-col gap-2", className)}
      {...props}
    >
      {backLink ? <div className="text-sm">{backLink}</div> : null}
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
        <div className="flex min-w-0 flex-1 flex-col">
          <h1 className={cn(TITLE_BASE, TITLE_SIZE[size], titleClassName)}>
            {icon ? (
              <span aria-hidden="true" className="inline-flex shrink-0 items-center">
                {icon}
              </span>
            ) : null}
            <span className="min-w-0">{title}</span>
          </h1>
          {subtitle ? (
            <p className={cn("mt-1 text-muted-foreground", subtitleClassName)}>{subtitle}</p>
          ) : null}
        </div>
        {actions ? (
          <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
        ) : null}
      </div>
    </header>
  )
}
