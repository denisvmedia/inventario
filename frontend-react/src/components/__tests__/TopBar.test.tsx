import { describe, expect, it } from "vitest"
import { screen } from "@testing-library/react"

import { SidebarProvider } from "@/components/ui/sidebar"
import { TopBar } from "@/components/TopBar"
import { RouteTitle, RouteTitleProvider } from "@/components/routing/RouteTitle"
import { renderWithProviders } from "@/test/render"

describe("TopBar", () => {
  it("displays the active route title from RouteTitleContext", async () => {
    renderWithProviders({
      children: (
        <RouteTitleProvider>
          <SidebarProvider>
            <RouteTitle title="Test Page" />
            <TopBar />
          </SidebarProvider>
        </RouteTitleProvider>
      ),
    })
    expect(await screen.findByTestId("topbar-title")).toHaveTextContent("Test Page")
  })

  it("includes the sidebar trigger and theme/density toggles", () => {
    renderWithProviders({
      children: (
        <RouteTitleProvider>
          <SidebarProvider>
            <TopBar />
          </SidebarProvider>
        </RouteTitleProvider>
      ),
    })
    expect(screen.getByRole("button", { name: /toggle sidebar/i })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /toggle theme/i })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /toggle density/i })).toBeInTheDocument()
  })
})
