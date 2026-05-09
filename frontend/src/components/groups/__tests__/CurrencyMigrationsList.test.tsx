import { describe, expect, it } from "vitest"
import { screen } from "@testing-library/react"

import { CurrencyMigrationsList } from "@/components/groups/CurrencyMigrationsList"
import { CurrencyMigrationStatusBadge } from "@/components/groups/CurrencyMigrationStatusBadge"
import type { Migration } from "@/features/currency-migration/api"
import { renderWithProviders } from "@/test/render"

const fixture: Migration[] = [
  {
    id: "m1",
    from_currency: "USD",
    to_currency: "EUR",
    exchange_rate: 0.9,
    status: "completed",
    created_at: "2026-05-08T12:00:00Z",
    commodity_count: 12,
  },
  {
    id: "m2",
    from_currency: "EUR",
    to_currency: "CZK",
    exchange_rate: 25,
    status: "running",
    created_at: "2026-05-08T11:00:00Z",
  },
  {
    id: "m3",
    from_currency: "CZK",
    to_currency: "USD",
    exchange_rate: 0.04,
    status: "failed",
    created_at: "2026-05-08T10:00:00Z",
  },
]

describe("<CurrencyMigrationsList />", () => {
  it("renders skeleton rows while loading", () => {
    renderWithProviders({
      children: <CurrencyMigrationsList loading={true} migrations={[]} />,
    })
    expect(screen.getByTestId("migrations-loading")).toBeInTheDocument()
  })

  it("renders the empty-state copy when there are no migrations", () => {
    renderWithProviders({
      children: <CurrencyMigrationsList loading={false} migrations={[]} />,
    })
    expect(screen.getByTestId("migrations-empty")).toBeInTheDocument()
  })

  it("renders one row per migration with from→to and the status badge", () => {
    renderWithProviders({
      children: <CurrencyMigrationsList loading={false} migrations={fixture} />,
    })
    expect(screen.getByTestId("migrations-list")).toBeInTheDocument()
    for (const m of fixture) {
      const row = screen.getByTestId(`migration-row-${m.id}`)
      expect(row).toHaveTextContent(m.from_currency!)
      expect(row).toHaveTextContent(m.to_currency!)
    }
    // Each terminal/non-terminal status badge renders with its own
    // data-testid so the e2e selectors can pick out the right pill.
    expect(screen.getByTestId("migration-status-completed")).toBeInTheDocument()
    expect(screen.getByTestId("migration-status-running")).toBeInTheDocument()
    expect(screen.getByTestId("migration-status-failed")).toBeInTheDocument()
  })

  it("caps display at 10 rows even when more are passed in", () => {
    const many: Migration[] = Array.from({ length: 25 }, (_, i) => ({
      id: `m${i}`,
      from_currency: "USD",
      to_currency: "EUR",
      exchange_rate: 0.9,
      status: "completed",
    }))
    renderWithProviders({
      children: <CurrencyMigrationsList loading={false} migrations={many} />,
    })
    // The component slices to 10; rows beyond the cap shouldn't render.
    expect(screen.queryByTestId("migration-row-m9")).toBeInTheDocument()
    expect(screen.queryByTestId("migration-row-m10")).toBeNull()
  })
})

describe("<CurrencyMigrationStatusBadge />", () => {
  it.each(["pending", "running", "completed", "failed"] as const)(
    "renders the %s pill with the matching i18n label",
    (status) => {
      renderWithProviders({
        children: <CurrencyMigrationStatusBadge status={status} />,
      })
      const badge = screen.getByTestId(`migration-status-${status}`)
      expect(badge).toBeInTheDocument()
      // The visible text varies by locale; we only assert the pill is
      // non-empty and carries the right testid (the i18n test guards
      // the text → key mapping separately).
      expect(badge.textContent?.trim().length ?? 0).toBeGreaterThan(0)
    }
  )

  it("threads className through to the underlying Badge", () => {
    renderWithProviders({
      children: <CurrencyMigrationStatusBadge status="completed" className="extra-class" />,
    })
    expect(screen.getByTestId("migration-status-completed")).toHaveClass("extra-class")
  })
})
