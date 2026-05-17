import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  createSupplyLink,
  deleteSupplyLink,
  listSupplyLinks,
  reorderSupplyLinks,
  updateSupplyLink,
  type CreateSupplyLinkRequest,
  type DeleteSupplyLinkRequest,
  type ListSupplyLinksResult,
  type ReorderSupplyLinksRequest,
  type SupplyLinkEntity,
  type UpdateSupplyLinkRequest,
} from "./api"
import { supplyLinkKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useSupplyLinksForCommodity(
  commodityID: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<ListSupplyLinksResult>({
    queryKey: supplyLinkKeys.byCommodity(slug, commodityID ?? ""),
    queryFn: ({ signal }) => {
      if (!commodityID) {
        throw new Error("useSupplyLinksForCommodity called without a commodity id")
      }
      return listSupplyLinks(commodityID, signal)
    },
    enabled: enabled && !!commodityID,
  })
}

function useInvalidateForCommodity() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return (commodityID: string) =>
    qc.invalidateQueries({ queryKey: supplyLinkKeys.byCommodity(slug, commodityID) })
}

export function useCreateSupplyLink() {
  const invalidate = useInvalidateForCommodity()
  return useMutation<SupplyLinkEntity & { id: string }, Error, CreateSupplyLinkRequest>({
    mutationFn: createSupplyLink,
    onSuccess: (_link, vars) => {
      invalidate(vars.commodity_id)
    },
  })
}

export function useUpdateSupplyLink() {
  const invalidate = useInvalidateForCommodity()
  return useMutation<SupplyLinkEntity & { id: string }, Error, UpdateSupplyLinkRequest>({
    mutationFn: updateSupplyLink,
    onSuccess: (_link, vars) => {
      invalidate(vars.commodity_id)
    },
  })
}

export function useDeleteSupplyLink() {
  const invalidate = useInvalidateForCommodity()
  return useMutation<void, Error, DeleteSupplyLinkRequest>({
    mutationFn: deleteSupplyLink,
    onSuccess: (_void, vars) => {
      invalidate(vars.commodity_id)
    },
  })
}

export function useReorderSupplyLinks() {
  const invalidate = useInvalidateForCommodity()
  return useMutation<ListSupplyLinksResult, Error, ReorderSupplyLinksRequest>({
    mutationFn: reorderSupplyLinks,
    onSuccess: (_result, vars) => {
      invalidate(vars.commodity_id)
    },
  })
}
