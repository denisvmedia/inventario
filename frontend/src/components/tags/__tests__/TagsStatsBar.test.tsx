import { axe } from "jest-axe"
import { render, screen } from "@testing-library/react"
import { beforeAll, describe, expect, it } from "vitest"

import { TagsStatsBar } from "@/components/tags/TagsStatsBar"
import { initI18n } from "@/i18n"

const STATS = {
  tags_total: 12,
  commodity_tags_total: 8,
  file_tags_total: 4,
  items_tagged: 50,
  items_untagged: 7,
  files_tagged: 30,
  files_untagged: 5,
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<TagsStatsBar />", () => {
  it("renders the commodity tiles for the item-tags view", () => {
    render(<TagsStatsBar kind="commodity" stats={STATS} />)
    expect(screen.getByTestId("tags-stats-commodity-tags-total")).toHaveTextContent("8")
    expect(screen.getByTestId("tags-stats-items-tagged")).toHaveTextContent("50")
    expect(screen.getByTestId("tags-stats-items-untagged")).toHaveTextContent("7")
    // File tiles must not appear in the commodity view.
    expect(screen.queryByTestId("tags-stats-files-tagged")).not.toBeInTheDocument()
  })

  it("renders the file tiles for the file-tags view", () => {
    render(<TagsStatsBar kind="file" stats={STATS} />)
    expect(screen.getByTestId("tags-stats-file-tags-total")).toHaveTextContent("4")
    expect(screen.getByTestId("tags-stats-files-tagged")).toHaveTextContent("30")
    expect(screen.getByTestId("tags-stats-files-untagged")).toHaveTextContent("5")
    expect(screen.queryByTestId("tags-stats-items-tagged")).not.toBeInTheDocument()
  })

  it("falls back to em-dashes while loading without data", () => {
    render(<TagsStatsBar kind="commodity" loading />)
    expect(screen.getByTestId("tags-stats-commodity-tags-total")).toHaveTextContent("—")
  })

  it("is axe-clean with stats present", async () => {
    const { container } = render(<TagsStatsBar kind="commodity" stats={STATS} />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
