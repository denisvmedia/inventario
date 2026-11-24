import { beforeEach, describe, expect, it } from "vitest"
import { useState } from "react"
import { Route } from "react-router-dom"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { TagPalette } from "@/components/tags/TagPalette"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { groupHandlers, tagHandlers } from "@/test/handlers"

const SLUG = "household"

function Harness({ initial }: { initial: string[] }) {
  const [values, setValues] = useState(initial)
  return (
    <>
      <TagPalette
        kind="commodity"
        values={values}
        onChange={setValues}
        inputId="tags-input"
        testId="palette"
      />
      <output data-testid="values">{values.join(",")}</output>
    </>
  )
}

function renderPalette(initial: string[] = []) {
  setAccessToken("good-token")
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
    ...tagHandlers.list(SLUG, [
      { id: "t1", slug: "work", label: "Work", color: "blue" },
      { id: "t2", slug: "garden-tools", label: "Garden tools", color: "green" },
    ])
  )
  return renderWithProviders({
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <Harness initial={initial} />
          </GroupProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<TagPalette />", () => {
  it("offers every tag of the group and toggles one per click", async () => {
    const user = userEvent.setup()
    renderPalette(["work"])
    const work = await screen.findByTestId("palette-option-work")
    const garden = screen.getByTestId("palette-option-garden-tools")
    expect(work).toHaveTextContent("#Work")
    expect(work).toHaveAttribute("aria-pressed", "true")
    expect(garden).toHaveAttribute("aria-pressed", "false")

    await user.click(garden)
    expect(screen.getByTestId("values")).toHaveTextContent("work,garden-tools")
    await user.click(work)
    expect(screen.getByTestId("values")).toHaveTextContent(/^garden-tools$/)
  })

  it("removes a selected tag from its pill", async () => {
    const user = userEvent.setup()
    renderPalette(["work", "garden-tools"])
    await screen.findByTestId("palette-option-work")
    await user.click(screen.getByRole("button", { name: "Remove tag Garden tools" }))
    expect(screen.getByTestId("values")).toHaveTextContent(/^work$/)
    expect(screen.queryByTestId("palette-selected-garden-tools")).toBeNull()
  })

  it("adds a new tag from the input, and picks an existing one by its label", async () => {
    const user = userEvent.setup()
    renderPalette()
    await screen.findByTestId("palette-option-work")
    const input = screen.getByRole("textbox", { name: "New tag" })
    expect(screen.getByTestId("palette-add")).toBeDisabled()

    await user.type(input, "fragile{Enter}")
    expect(screen.getByTestId("values")).toHaveTextContent(/^fragile$/)
    expect(input).toHaveValue("")

    await user.type(input, "garden TOOLS")
    await user.click(screen.getByTestId("palette-add"))
    expect(screen.getByTestId("values")).toHaveTextContent(/^fragile,garden-tools$/)
    expect(screen.getByTestId("palette-option-garden-tools")).toHaveAttribute(
      "aria-pressed",
      "true"
    )
  })

  it("has no axe violations", async () => {
    const { container } = renderPalette(["work"])
    await screen.findByTestId("palette-option-work")
    expect(await axe(container)).toHaveNoViolations()
  })
})
