import { Button } from "@/components/ui/button"

import { RouteTitle } from "@/components/routing/RouteTitle"

// DashboardPage is the bare /g/:groupSlug/ index. It renders group-scoped
// totals (counts, recent additions, expiring warranties) — the actual data
// lands in #1408. For now it's a placeholder so the rest of the routing
// foundation has a real component to mount.
export function DashboardPage() {
  return (
    <>
      <RouteTitle title="Dashboard" />
      <section
        aria-labelledby="dashboard-title"
        className="flex flex-col gap-4 max-w-md w-full text-center"
      >
        <h1 id="dashboard-title" className="scroll-m-20 text-3xl font-semibold tracking-tight">
          Inventario
        </h1>
        <p className="text-muted-foreground text-sm">
          Dashboard scaffold — group-scoped widgets land in #1408 under epic #1397.
        </p>
        <div className="flex justify-center pt-2">
          <Button variant="default" size="default">
            Get started
          </Button>
        </div>
      </section>
    </>
  )
}
