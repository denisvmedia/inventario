import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

interface RouteTitleContextValue {
  title: string
  setTitle: (next: string) => void
}

const RouteTitleContext = createContext<RouteTitleContextValue | undefined>(undefined)

interface RouteTitleProviderProps {
  children: ReactNode
}

// RouteTitleProvider holds the active page's translated title in a React
// context so the top bar (and any future breadcrumb) can show what the user
// is looking at without each page having to thread a string up through the
// router. RouteTitle is the only writer; consumers read via
// useCurrentRouteTitle().
export function RouteTitleProvider({ children }: RouteTitleProviderProps) {
  const [title, setTitle] = useState("")
  const value = useMemo(() => ({ title, setTitle }), [title])
  return <RouteTitleContext.Provider value={value}>{children}</RouteTitleContext.Provider>
}

export function useCurrentRouteTitle(): string {
  const ctx = useContext(RouteTitleContext)
  // Outside the provider — used in tests that mount RouteTitle in isolation.
  // Returning "" rather than throwing keeps those legacy callsites working
  // while the provider is the production path.
  return ctx?.title ?? ""
}

interface RouteTitleProps {
  // The translated page name (caller resolves via useTranslation()). Keeping
  // this as a plain string rather than a key+ns lets pages compute the
  // string once and reuse it for both the heading and the title.
  title: string
  // Override the default brand suffix. Empty string omits the separator
  // entirely (rare — used by full-screen takeover pages that want just a
  // verb in the tab).
  suffix?: string
}

// RouteTitle keeps document.title in sync with the rendered page AND
// broadcasts the bare title (no brand suffix) into RouteTitleContext so
// top-bar / breadcrumbs can render it.
//
// Place inside each <Route element>. The previous title isn't restored on
// unmount because the next route mounts its own RouteTitle in the same
// render pass — trying to restore would race the new page's update.
export function RouteTitle({ title, suffix }: RouteTitleProps) {
  const { t } = useTranslation()
  const ctx = useContext(RouteTitleContext)
  const brand = suffix ?? t("common:brand")
  useEffect(() => {
    document.title = brand ? t("common:documentTitle", { title, brand }) : title
    ctx?.setTitle(title)
  }, [title, brand, t, ctx])
  return null
}
