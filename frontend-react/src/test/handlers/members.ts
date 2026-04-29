import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Members live under /groups/{id}/members rather than /g/{slug}/...
// (the legacy backend keeps the membership endpoints group-id-keyed).
export function list(groupId: string, members: unknown[] = []) {
  return [
    http.get(apiUrl(`/groups/${encodeURIComponent(groupId)}/members`), () =>
      HttpResponse.json({ data: members })
    ),
  ]
}
