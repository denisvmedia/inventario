import { http, HttpResponse } from "msw"

import type { Schema } from "@/types"

import { apiUrl } from "."

type LocationGroup = Schema<"models.LocationGroup">

export const fixtureGroups: LocationGroup[] = [
  { id: "g1", slug: "household", name: "Household" } as LocationGroup,
  { id: "g2", slug: "office", name: "Office" } as LocationGroup,
]

function envelope(groups: LocationGroup[]) {
  return {
    data: groups.map((g) => ({ id: g.id, type: "groups", attributes: g })),
  }
}

// list builds a /groups handler that returns the supplied membership.
// Defaults to the two-group fixture so the common "user with multiple
// groups" path is a one-liner in tests.
export function list(groups: LocationGroup[] = fixtureGroups) {
  return [http.get(apiUrl("/groups"), () => HttpResponse.json(envelope(groups)))]
}

export function empty() {
  return [http.get(apiUrl("/groups"), () => HttpResponse.json(envelope([])))]
}

export function error(status = 500) {
  return [http.get(apiUrl("/groups"), () => HttpResponse.json({ error: "boom" }, { status }))]
}
