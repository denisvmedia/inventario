import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { commodityKeys } from "@/features/commodities/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"

import {
  createSeller,
  deleteSeller,
  listSellers,
  updateSeller,
  type Seller,
  type SellerInput,
} from "./api"
import { sellerKeys } from "./keys"

// useSellers lists the group's sellers sorted by name, each with its item
// count. It uses the optional group hook so the item form can render it in
// harnesses without a group; outside a group the query stays off.
export function useSellers() {
  const slug = useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
  return useQuery<Seller[]>({
    queryKey: sellerKeys.list(slug),
    queryFn: ({ signal }) => listSellers(signal),
    enabled: slug.length > 0,
    placeholderData: (prev) => prev,
  })
}

function useInvalidate() {
  const qc = useQueryClient()
  const slug = useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
  return {
    sellers: () => qc.invalidateQueries({ queryKey: sellerKeys.group(slug) }),
    // A deleted seller is unlinked from its items, so the cached items
    // still name it until they are fetched again.
    sellersAndItems: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: sellerKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
      ]),
  }
}

export function useCreateSeller() {
  const invalidate = useInvalidate()
  return useMutation<Seller, Error, SellerInput>({
    mutationFn: (input) => createSeller(input),
    onSuccess: () => invalidate.sellers(),
  })
}

export function useUpdateSeller() {
  const invalidate = useInvalidate()
  return useMutation<Seller, Error, { id: string; input: Partial<SellerInput> }>({
    mutationFn: ({ id, input }) => updateSeller(id, input),
    onSuccess: () => invalidate.sellers(),
  })
}

export function useDeleteSeller() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, string>({
    mutationFn: (id) => deleteSeller(id),
    onSuccess: () => invalidate.sellersAndItems(),
  })
}
