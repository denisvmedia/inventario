import { useEffect } from "react"

interface RouteTitleProps {
  title: string
  // Default suffix is the brand name; pages typically just pass `title`
  // and we append " · Inventario" so tabs are scannable.
  suffix?: string
}

// RouteTitle keeps document.title in sync with the rendered page. Place it
// inside each <Route element>. The previous title isn't restored on unmount
// because the next route mounts its own RouteTitle in the same render pass —
// trying to restore would race the new page's update.
export function RouteTitle({ title, suffix = "Inventario" }: RouteTitleProps) {
  useEffect(() => {
    document.title = suffix ? `${title} · ${suffix}` : title
  }, [title, suffix])
  return null
}
