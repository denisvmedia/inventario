import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, within } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { InsuranceReportPage } from "@/pages/reports/InsuranceReportPage"
import { useAreas } from "@/features/areas/hooks"
import { useCommodities, useCommodity } from "@/features/commodities/hooks"
import { useFiles } from "@/features/files/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useLocations } from "@/features/locations/hooks"

// The report page reads from a handful of feature hooks; mock them all so
// the test drives pure presentation off fixtures (mirrors the
// CommodityDetailPage test harness).
vi.mock("@/features/commodities/hooks", () => ({
  useCommodities: vi.fn(),
  useCommodity: vi.fn(),
}))
vi.mock("@/features/areas/hooks", () => ({ useAreas: vi.fn() }))
vi.mock("@/features/locations/hooks", () => ({ useLocations: vi.fn() }))
vi.mock("@/features/files/hooks", () => ({ useFiles: vi.fn() }))
vi.mock("@/features/group/GroupContext", async () => {
  const actual = await vi.importActual<object>("@/features/group/GroupContext")
  return { ...actual, useCurrentGroup: vi.fn() }
})

const mockUseCommodities = vi.mocked(useCommodities)
const mockUseCommodity = vi.mocked(useCommodity)
const mockUseAreas = vi.mocked(useAreas)
const mockUseLocations = vi.mocked(useLocations)
const mockUseFiles = vi.mocked(useFiles)
const mockUseCurrentGroup = vi.mocked(useCurrentGroup)

const baseGroup = {
  id: "g1",
  slug: "test-group",
  name: "Test Group",
  group_currency: "USD",
}

function renderPage(search: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/g/test-group/reports/insurance${search}`]}>
        <Routes>
          <Route path="/g/:groupSlug/reports/insurance" element={<InsuranceReportPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

const itemC1 = {
  id: "c1",
  name: "Espresso Machine",
  short_name: "Gaggia",
  type: "electronics",
  serial_number: "SN-12345",
  purchase_date: "2023-06-15",
  original_price: 700,
  original_price_currency: "USD",
  converted_original_price: 700,
  current_price: 650,
  warranty_expires_at: "2026-06-15",
  warranty_notes: "Manufacturer 3-year",
  area_id: "a1",
}
const itemC2 = {
  id: "c2",
  name: "Toaster",
  type: "appliances",
  converted_original_price: 50,
  current_price: 40,
  area_id: "a1",
}

const areas = [{ id: "a1", name: "Kitchen Counter", location_id: "loc1" }]
const locations = [{ id: "loc1", name: "Kitchen" }]

function baseHooks() {
  mockUseCurrentGroup.mockReturnValue({
    currentGroup: baseGroup,
  } as unknown as ReturnType<typeof useCurrentGroup>)
  mockUseAreas.mockReturnValue({
    data: areas,
    isLoading: false,
    isError: false,
  } as unknown as ReturnType<typeof useAreas>)
  mockUseLocations.mockReturnValue({
    data: locations,
    isLoading: false,
    isError: false,
  } as unknown as ReturnType<typeof useLocations>)
  mockUseFiles.mockReturnValue({
    data: { files: [], total: 0 },
    isLoading: false,
    isError: false,
  } as unknown as ReturnType<typeof useFiles>)
}

afterEach(() => {
  vi.clearAllMocks()
})

describe("<InsuranceReportPage /> — item mode", () => {
  beforeEach(() => {
    baseHooks()
    mockUseCommodities.mockReturnValue({
      data: { commodities: [itemC1, itemC2], total: 2, covers: {} },
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useCommodities>)
    mockUseCommodity.mockReturnValue({
      data: { commodity: itemC1, meta: {} },
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useCommodity>)
  })

  it("renders the selected item's report with its fields", () => {
    renderPage("?mode=item&item=c1")
    const report = screen.getByTestId("report-item")
    expect(within(report).getByText("Espresso Machine")).toBeInTheDocument()
    expect(within(report).getByText("Item Insurance Report")).toBeInTheDocument()
    // Type → commodities:type.* catalogue, serial, area breadcrumb.
    expect(within(report).getByText("SN-12345")).toBeInTheDocument()
    expect(within(report).getByText("Kitchen · Kitchen Counter")).toBeInTheDocument()
    // Currency uses the group / purchase currency.
    expect(within(report).getByText("$700.00")).toBeInTheDocument()
    expect(within(report).getByText("$650.00")).toBeInTheDocument()
    // Warranty notes surface.
    expect(within(report).getByText("Manufacturer 3-year")).toBeInTheDocument()
  })
})

describe("<InsuranceReportPage /> — location mode", () => {
  beforeEach(() => {
    baseHooks()
    mockUseCommodities.mockReturnValue({
      data: { commodities: [itemC1, itemC2], total: 2, covers: {} },
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useCommodities>)
    mockUseCommodity.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useCommodity>)
  })

  it("renders each item and the correct location totals", () => {
    renderPage("?mode=location&location=loc1")
    const report = screen.getByTestId("report-location")
    expect(within(report).getByText("Location Insurance Report")).toBeInTheDocument()
    // Both items in the location render as per-item sections.
    expect(within(report).getAllByTestId("report-location-item")).toHaveLength(2)
    expect(within(report).getByText("Espresso Machine")).toBeInTheDocument()
    expect(within(report).getByText("Toaster")).toBeInTheDocument()
    // Totals: count 2, purchase 700+50=750, value 650+40=690.
    expect(within(report).getByText("$750.00")).toBeInTheDocument()
    expect(within(report).getByText("$690.00")).toBeInTheDocument()
  })

  it("shows the empty state when the location has no items", () => {
    mockUseCommodities.mockReturnValue({
      data: { commodities: [], total: 0, covers: {} },
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useCommodities>)
    renderPage("?mode=location&location=loc1")
    expect(screen.getByTestId("report-location-empty")).toBeInTheDocument()
    expect(screen.getByText("No items found for this location.")).toBeInTheDocument()
  })
})
