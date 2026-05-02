import { describe, expect, it, beforeEach } from "vitest"
import { render } from "@testing-library/react"

import { RouteTitle } from "@/components/routing/RouteTitle"

beforeEach(() => {
  document.title = "initial"
})

describe("RouteTitle", () => {
  it("sets document.title with the brand suffix appended", () => {
    render(<RouteTitle title="Locations" />)
    expect(document.title).toBe("Locations · Inventario")
  })

  it("honors a custom suffix", () => {
    render(<RouteTitle title="Settings" suffix="Custom" />)
    expect(document.title).toBe("Settings · Custom")
  })

  it("drops the separator when suffix is empty", () => {
    render(<RouteTitle title="Standalone" suffix="" />)
    expect(document.title).toBe("Standalone")
  })
})
