import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { render, screen, waitFor, within } from "@testing-library/react"
import { axe } from "jest-axe"
import whatsNewMeta from "virtual:whats-new/meta"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { parseChangelog, type WhatsNewMeta } from "@/lib/changelog"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { WhatsNewPage, WhatsNewView } from "@/pages/WhatsNewPage"
import { authHandlers, groupHandlers, settingsHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const RELEASES = parseChangelog(
  `## [0.3.0] - 2026-12-01

### Added

- A **Views** menu ([#12](https://github.com/acme/app/pull/12)).

### Fixed

- The \`/items\` page.

### Operators

- A Helm value.

## [0.2.2] - 2026-11-23

### Operators

- A chart fix.

## [0.1.0] - 2026-09-19

First release.
`,
  { linkBase: "https://github.com/acme/app/blob/master/" }
)

const RELEASE_BUILD: WhatsNewMeta = { build: { kind: "release", version: "0.3.0" }, latestVersion: "0.3.0" }
const DEV_BUILD: WhatsNewMeta = {
  build: { kind: "development", label: "edge", base: null },
  latestVersion: "0.3.0",
}

describe("<WhatsNewView />", () => {
  it("lists the releases newest first with their date and sections", () => {
    render(<WhatsNewView meta={RELEASE_BUILD} releases={RELEASES} showOperators={false} />)
    const cards = within(screen.getByTestId("whats-new-releases")).getAllByRole("region")
    expect(cards.map((c) => within(c).getByRole("heading", { level: 2 }).textContent)).toEqual([
      "v0.3.0",
      "v0.2.2",
      "v0.1.0",
    ])
    const latest = screen.getByTestId("whats-new-release-0.3.0")
    expect(within(latest).getByText("December 1, 2026")).toBeInTheDocument()
    expect(
      within(latest)
        .getAllByRole("heading", { level: 3 })
        .map((h) => h.textContent)
    ).toEqual(["Added", "Fixed"])
    expect(within(latest).getByText("Views").tagName).toBe("STRONG")
    expect(within(latest).getByText("/items").tagName).toBe("CODE")
    expect(screen.getByText("First release.")).toBeInTheDocument()
    expect(screen.queryByTestId("whats-new-development-build")).not.toBeInTheDocument()
  })

  it("opens the links of an entry in a new tab", () => {
    render(<WhatsNewView meta={RELEASE_BUILD} releases={RELEASES} showOperators={false} />)
    const link = screen.getByRole("link", { name: "#12" })
    expect(link).toHaveAttribute("href", "https://github.com/acme/app/pull/12")
    expect(link).toHaveAttribute("target", "_blank")
    expect(link.getAttribute("rel")).toContain("noopener")
  })

  it("hides the Operators section from users who are not system administrators", () => {
    render(<WhatsNewView meta={RELEASE_BUILD} releases={RELEASES} showOperators={false} />)
    expect(screen.queryByTestId("whats-new-section-operators")).not.toBeInTheDocument()
    expect(screen.queryByText("A Helm value.")).not.toBeInTheDocument()
    // A release that changed only the deployment says so.
    const ops = screen.getByTestId("whats-new-release-0.2.2")
    expect(within(ops).getByTestId("whats-new-no-user-changes")).toBeInTheDocument()
  })

  it("shows the Operators section, last, to system administrators", () => {
    render(<WhatsNewView meta={RELEASE_BUILD} releases={RELEASES} showOperators />)
    const latest = screen.getByTestId("whats-new-release-0.3.0")
    expect(
      within(latest)
        .getAllByRole("heading", { level: 3 })
        .map((h) => h.textContent)
    ).toEqual(["Added", "Fixed", "Operators"])
    expect(within(latest).getByText("A Helm value.")).toBeInTheDocument()
    expect(within(latest).getByText("Shown to system administrators only.")).toBeInTheDocument()
    expect(screen.queryByTestId("whats-new-no-user-changes")).not.toBeInTheDocument()
  })

  it("says when the server runs a development build", () => {
    render(<WhatsNewView meta={DEV_BUILD} releases={RELEASES} showOperators={false} />)
    expect(screen.getByTestId("whats-new-development-build")).toHaveTextContent(
      "This server runs edge, which is not a release."
    )
  })

  it("has an empty state without releases", () => {
    render(
      <WhatsNewView
        meta={{ ...DEV_BUILD, latestVersion: null }}
        releases={[]}
        showOperators={false}
      />
    )
    expect(screen.getByTestId("whats-new-empty")).toHaveTextContent("No release has been published yet.")
  })

  it("has no axe violations", async () => {
    const { container } = render(
      <WhatsNewView meta={DEV_BUILD} releases={RELEASES} showOperators />
    )
    expect(await axe(container)).toHaveNoViolations()
  })
})

describe("<WhatsNewPage />", () => {
  beforeEach(() => {
    clearAuth()
    __resetGroupContextForTests()
    __resetHttpForTests()
  })

  function renderPage() {
    setAccessToken("good-token")
    return renderWithProviders({
      initialPath: "/whats-new?g=household",
      routes: (
        <Route
          path="/whats-new"
          element={
            <AuthProvider>
              <GroupProvider>
                <WhatsNewPage />
              </GroupProvider>
            </AuthProvider>
          }
        />
      ),
    })
  }

  it("reads the releases of CHANGELOG.md and marks the newest as seen", async () => {
    // Tests run as a "dev" build, which keeps every released version.
    expect(whatsNewMeta.build.kind).toBe("development")
    expect(whatsNewMeta.latestVersion).toMatch(/^\d+\.\d+\.\d+/)
    const fake = settingsHandlers.backend("household")
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(), ...fake.handlers)
    renderPage()
    expect(
      await screen.findByTestId(`whats-new-release-${whatsNewMeta.latestVersion}`)
    ).toBeInTheDocument()
    expect(screen.getByTestId("whats-new-development-build")).toBeInTheDocument()
    await waitFor(() =>
      expect(fake.patches).toEqual([
        { field: "whats_new.last_seen_version", value: whatsNewMeta.latestVersion },
      ])
    )
  })

  it("leaves the setting alone when the newest release was seen", async () => {
    const fake = settingsHandlers.backend("household", {
      whatsNewLastSeenVersion: whatsNewMeta.latestVersion ?? undefined,
    })
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(), ...fake.handlers)
    renderPage()
    await screen.findByTestId("whats-new-page")
    await waitFor(() => expect(fake.reads).toBeGreaterThan(0))
    await new Promise((r) => setTimeout(r, 50))
    expect(fake.patches).toEqual([])
  })

  it("shows the Operators sections to a system administrator only", async () => {
    // 0.2.0 has an Operators section in CHANGELOG.md.
    const fake = settingsHandlers.backend("household")
    server.use(
      ...authHandlers.signedIn({ user: { is_system_admin: true } }),
      ...groupHandlers.list(),
      ...fake.handlers
    )
    const { unmount } = renderPage()
    const release = await screen.findByTestId("whats-new-release-0.2.0")
    expect(await within(release).findByTestId("whats-new-section-operators")).toBeInTheDocument()
    unmount()

    clearAuth()
    __resetGroupContextForTests()
    __resetHttpForTests()
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(), ...fake.handlers)
    renderPage()
    const plain = await screen.findByTestId("whats-new-release-0.2.0")
    expect(within(plain).queryByTestId("whats-new-section-operators")).not.toBeInTheDocument()
    expect(within(plain).getByTestId("whats-new-section-fixed")).toBeInTheDocument()
  })
})
