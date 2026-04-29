import { useQuery } from "@tanstack/react-query"

import { listGroups, type LocationGroup } from "./api"
import { groupKeys } from "./keys"

// Fetches the user's active groups. The list is cheap, doesn't change often,
// and is read by the GroupProvider, the GroupRequiredRoute guard, and the
// group switcher in the app shell — one cache entry serves all three.
export function useGroups() {
  return useQuery<LocationGroup[]>({
    queryKey: groupKeys.list(),
    queryFn: ({ signal }) => listGroups(signal),
  })
}
