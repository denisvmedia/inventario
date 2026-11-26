import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useCallback } from "react"

import { commodityKeys } from "@/features/commodities/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { getServerErrorCode } from "@/lib/server-error"

import {
  createCustomField,
  CUSTOM_FIELD_NAME_TAKEN,
  deleteCustomField,
  listCustomFields,
  updateCustomField,
  type CustomField,
  type CustomFieldInput,
} from "./api"
import { customFieldKeys } from "./keys"

// Fields change rarely and every item form and item page reads them, so a
// fetched list stays fresh for a minute instead of refetching on each mount.
const CUSTOM_FIELDS_STALE_MS = 60_000

// useCustomFields lists the group's custom fields with their item counts.
// Without a group (the landing page's first item) the query stays off. A
// component that is handed its group's slug instead of reading the group
// context (the AI scan step) passes it as slug.
export function useCustomFields({ slug: slugProp }: { slug?: string } = {}) {
  const contextSlug = useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
  const slug = slugProp ?? contextSlug
  return useQuery<CustomField[]>({
    queryKey: customFieldKeys.list(slug),
    queryFn: ({ signal }) => listCustomFields(signal),
    enabled: slug.length > 0,
    staleTime: CUSTOM_FIELDS_STALE_MS,
  })
}

// useCustomFieldName returns the name of a field of the current group by
// id, undefined for one this client does not know.
export function useCustomFieldName(): (id: string) => string | undefined {
  const data = useCustomFields().data
  return useCallback((id: string) => data?.find((f) => f.id === id)?.name, [data])
}

function useInvalidate() {
  const qc = useQueryClient()
  const slug = useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
  return {
    fields: () => qc.invalidateQueries({ queryKey: customFieldKeys.group(slug) }),
    // Removing an option or deleting a field clears values, so the cached
    // items still show them until they are fetched again.
    fieldsAndItems: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: customFieldKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
      ]),
  }
}

// A name refused as taken belongs to a field this client may not know yet,
// so the list is fetched again.
function refreshOnNameTaken(err: Error, refresh: () => unknown) {
  if (getServerErrorCode(err) === CUSTOM_FIELD_NAME_TAKEN) void refresh()
}

export function useCreateCustomField() {
  const invalidate = useInvalidate()
  return useMutation<CustomField, Error, CustomFieldInput>({
    mutationFn: (input) => createCustomField(input),
    onSuccess: () => invalidate.fields(),
    onError: (err) => refreshOnNameTaken(err, invalidate.fields),
  })
}

export function useUpdateCustomField() {
  const invalidate = useInvalidate()
  return useMutation<CustomField, Error, { id: string; input: Partial<CustomFieldInput> }>({
    mutationFn: ({ id, input }) => updateCustomField(id, input),
    onSuccess: () => invalidate.fieldsAndItems(),
    onError: (err) => refreshOnNameTaken(err, invalidate.fields),
  })
}

export function useDeleteCustomField() {
  const invalidate = useInvalidate()
  return useMutation<number, Error, { id: string; deleteValues: boolean; expectedValues?: number }>(
    {
      mutationFn: ({ id, deleteValues, expectedValues }) =>
        deleteCustomField(id, deleteValues, expectedValues),
      onSuccess: () => invalidate.fieldsAndItems(),
      // A refused delete means the counts this client showed were stale.
      onError: () => invalidate.fields(),
    }
  )
}
