import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import type { LucideIcon } from "lucide-react"
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { commodityKeys } from "@/features/commodities/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { getServerErrorCode } from "@/lib/server-error"

import {
  createItemType,
  deleteItemType,
  ITEM_TYPE_NAME_TAKEN,
  listItemTypes,
  updateItemType,
  type ItemType,
  type ItemTypeInput,
} from "./api"
import { builtinItemTypes, isBuiltinItemTypeSlug, itemTypeIconFor, itemTypeLabel } from "./display"
import { itemTypeKeys } from "./keys"

// Types change rarely and every list, card and form shows them, so a
// fetched list stays fresh for a minute instead of refetching on each mount.
const ITEM_TYPES_STALE_MS = 60_000

// useItemTypes lists the group's item types with their item counts. It uses
// the optional group hook, so a component that shows a type also renders
// without a group: the query then stays off and, once it is clear there is
// no group, its data is the built-in types, which is what the item form of
// a visitor who has not signed up yet offers (#1988).
export function useItemTypes() {
  const group = useOptionalCurrentGroup()
  const slug = group?.currentGroup?.slug ?? ""
  const noGroup = !group || (!group.isLoading && !group.currentGroup)
  return useQuery<ItemType[]>({
    queryKey: itemTypeKeys.list(slug),
    queryFn: ({ signal }) => listItemTypes(signal),
    enabled: slug.length > 0,
    staleTime: ITEM_TYPES_STALE_MS,
    placeholderData: (prev) => prev ?? (noGroup ? builtinItemTypes() : undefined),
  })
}

export interface ItemTypeDisplay {
  slug: string
  label: string
  Icon: LucideIcon
  // The group's type, when the group has one with this slug.
  type?: ItemType
}

export interface ItemTypeLookup {
  // The group's types in the server's order; empty until they load.
  types: ItemType[]
  isLoaded: boolean
  // The label and icon of an item's type. A slug the group has no type for
  // (not loaded yet, or deleted meanwhile) shows the translated default of
  // a built-in slug, otherwise the slug itself.
  get: (slug: string | null | undefined) => ItemTypeDisplay | undefined
  // Whether an item may take the slug: a type of the group, or before the
  // list has loaded, a built-in slug.
  has: (slug: string | null | undefined) => boolean
}

// useItemTypeLookup resolves item type slugs to what the app shows for them.
export function useItemTypeLookup(): ItemTypeLookup {
  const { t } = useTranslation()
  const query = useItemTypes()
  const data = query.data
  return useMemo(() => {
    const types = data ?? []
    const bySlug = new Map(types.map((type) => [type.slug ?? "", type]))
    const isLoaded = data !== undefined
    return {
      types,
      isLoaded,
      get(slug) {
        if (!slug) return undefined
        const type = bySlug.get(slug)
        return {
          slug,
          label: itemTypeLabel(t, type ?? { slug, name: "" }),
          Icon: itemTypeIconFor(type, slug),
          type,
        }
      },
      has(slug) {
        if (!slug) return false
        return isLoaded ? bySlug.has(slug) : isBuiltinItemTypeSlug(slug)
      },
    }
  }, [data, t])
}

// useItemTypeFilterOptions lists what a type filter offers: the group's
// types, then any selected slug the group no longer has, so a filter left
// over in a URL can still be cleared.
export function useItemTypeFilterOptions(selected: readonly string[]): ItemTypeDisplay[] {
  const lookup = useItemTypeLookup()
  return useMemo(() => {
    const options = lookup.types
      .map((type) => lookup.get(type.slug))
      .filter((option): option is ItemTypeDisplay => option !== undefined)
    for (const slug of selected) {
      if (options.some((option) => option.slug === slug)) continue
      const option = lookup.get(slug)
      if (option) options.push(option)
    }
    return options
  }, [lookup, selected])
}

function useInvalidate() {
  const qc = useQueryClient()
  const slug = useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
  return {
    itemTypes: () => qc.invalidateQueries({ queryKey: itemTypeKeys.group(slug) }),
    // A delete moves the type's items to another type, so the cached items
    // still carry the old slug until they are fetched again.
    itemTypesAndItems: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: itemTypeKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
      ]),
  }
}

// A name refused as taken belongs to a type this client may not know yet,
// created by another member or in another tab, so the list is fetched again
// and the form can offer that type.
function refreshOnNameTaken(err: Error, refresh: () => unknown) {
  if (getServerErrorCode(err) === ITEM_TYPE_NAME_TAKEN) void refresh()
}

export function useCreateItemType() {
  const invalidate = useInvalidate()
  return useMutation<ItemType, Error, ItemTypeInput>({
    mutationFn: (input) => createItemType(input),
    onSuccess: () => invalidate.itemTypes(),
    onError: (err) => refreshOnNameTaken(err, invalidate.itemTypes),
  })
}

export function useUpdateItemType() {
  const invalidate = useInvalidate()
  return useMutation<ItemType, Error, { id: string; input: Partial<ItemTypeInput> }>({
    mutationFn: ({ id, input }) => updateItemType(id, input),
    onSuccess: () => invalidate.itemTypes(),
    onError: (err) => refreshOnNameTaken(err, invalidate.itemTypes),
  })
}

export function useDeleteItemType() {
  const invalidate = useInvalidate()
  return useMutation<number, Error, { id: string; reassignTo?: string }>({
    mutationFn: ({ id, reassignTo }) => deleteItemType(id, reassignTo),
    onSuccess: () => invalidate.itemTypesAndItems(),
    // A refused delete means the counts the page shows are stale.
    onError: () => invalidate.itemTypes(),
  })
}
