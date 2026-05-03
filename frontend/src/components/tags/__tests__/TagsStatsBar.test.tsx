import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import { beforeAll, describe, expect, it } from "vitest"

import { TagsStatsBar } from "@/components/tags/TagsStatsBar"
import { initI18n } from "@/i18n"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<TagsStatsBar />", () => {
  it("renders five tiles with the supplied counts", () => {
    render(
      <TagsStatsBar
        stats={{
          tags_total: 12,
          items_tagged: 50,
          items_untagged: 7,
          files_tagged: 30,
          files_untagged: 5,
        }}
      />
    )
    expect(screen.getByTestId("tags-stats-tags-total")).toHaveTextContent("12")
    expect(screen.getByTestId("tags-stats-items-tagged")).toHaveTextContent("50")
    expect(screen.getByTestId("tags-stats-items-untagged")).toHaveTextContent("7")
    expect(screen.getByTestId("tags-stats-files-tagged")).toHaveTextContent("30")
    expect(screen.getByTestId("tags-stats-files-untagged")).toHaveTextContent("5")
  })

  it("falls back to em-dashes while loading without data", () => {
    render(<TagsStatsBar loading />)
    expect(screen.getByTestId("tags-stats-tags-total")).toHaveTextContent("—")
  })

  it("is axe-clean with stats present", async () => {
    const { container } = render(
      <TagsStatsBar
        stats={{
          tags_total: 1,
          items_tagged: 0,
          items_untagged: 0,
          files_tagged: 0,
          files_untagged: 0,
        }}
      />
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
