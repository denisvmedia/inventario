import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"

import { MaintenancePage } from "./MaintenancePage"

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <MaintenancePage />
    </MemoryRouter>
  )
}

describe("MaintenancePage", () => {
  it("renders the heading + scheduled-maintenance badge + Status card with defaults when no headers are provided", () => {
    renderAt("/maintenance")
    expect(screen.getByRole("heading", { name: /right back/i, level: 1 })).toBeInTheDocument()
    expect(screen.getByTestId("maintenance-badge")).toHaveTextContent(/scheduled/i)
    // All three components default to "maintenance" when no per-component
    // status header is sent — the most conservative read.
    expect(screen.getByTestId("maintenance-status-api")).toHaveTextContent(/maintenance/i)
    expect(screen.getByTestId("maintenance-status-database")).toHaveTextContent(/maintenance/i)
    expect(screen.getByTestId("maintenance-status-storage")).toHaveTextContent(/maintenance/i)
  })

  it("renders per-component status colours from the X-Maintenance-Status URL param", () => {
    renderAt("/maintenance?status=api%3Ddegraded%2Cdatabase%3Dmaintenance%2Cstorage%3Doperational")
    expect(screen.getByTestId("maintenance-status-api")).toHaveTextContent(/degraded/i)
    expect(screen.getByTestId("maintenance-status-database")).toHaveTextContent(/maintenance/i)
    expect(screen.getByTestId("maintenance-status-storage")).toHaveTextContent(/operational/i)
  })

  it("renders the resume time when Retry-After delta-seconds is provided", () => {
    renderAt("/maintenance?retry_after=900")
    expect(screen.getByTestId("maintenance-resume")).toBeInTheDocument()
  })

  it("renders the resume time when Retry-After is an HTTP-date", () => {
    const futureIso = new Date(Date.now() + 60 * 60 * 1000).toUTCString()
    renderAt(`/maintenance?retry_after=${encodeURIComponent(futureIso)}`)
    expect(screen.getByTestId("maintenance-resume")).toBeInTheDocument()
  })

  it("hides the resume line on invalid Retry-After input", () => {
    renderAt("/maintenance?retry_after=not-a-date")
    expect(screen.queryByTestId("maintenance-resume")).not.toBeInTheDocument()
  })

  it("hides the resume line once the resume time has already passed", () => {
    const pastIso = new Date(Date.now() - 60 * 60 * 1000).toUTCString()
    renderAt(`/maintenance?retry_after=${encodeURIComponent(pastIso)}`)
    expect(screen.queryByTestId("maintenance-resume")).not.toBeInTheDocument()
  })
})
