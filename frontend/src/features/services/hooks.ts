import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  deleteService,
  getServiceCounts,
  listGroupServices,
  listServicesForCommodity,
  returnService,
  startService,
  updateService,
  type ListGroupServicesOptions,
  type ListedService,
  type ServiceEntity,
  type StartServiceRequest,
  type UpdateServiceRequest,
} from "./api"
import { serviceKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useServicesForCommodity(
  commodityID: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ services: Array<ServiceEntity & { id: string }>; total: number }>({
    queryKey: serviceKeys.byCommodity(slug, commodityID ?? ""),
    queryFn: ({ signal }) => {
      if (!commodityID) {
        throw new Error("useServicesForCommodity called without a commodity id")
      }
      return listServicesForCommodity(commodityID, signal)
    },
    // Gate on `slug` too: the http client rewrites /services →
    // /g/{slug}/services using `getCurrentGroupSlug()`, which the
    // GroupProvider populates in a useEffect after first paint. Without
    // the slug gate a cold mount of /g/:slug/commodities/:id fires the
    // first request before the effect resolves, gets back a 404, and the
    // tab spends a refresh cycle showing the empty state. See same
    // pattern on the auto-arming in tests (`setCurrentGroupSlug` in
    // beforeEach) for why.
    enabled: enabled && !!commodityID && !!slug,
  })
}

export function useGroupServices(opts: ListGroupServicesOptions = {}, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ services: ListedService[]; total: number }>({
    queryKey: serviceKeys.groupList(slug, opts),
    queryFn: ({ signal }) => listGroupServices({ ...opts, signal }),
    // See useServicesForCommodity — same group-slug gating rationale.
    enabled: (query.enabled ?? true) && !!slug,
    placeholderData: (prev) => prev,
  })
}

export function useServiceCounts(commodityIDs: string[], { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Record<string, number>>({
    queryKey: serviceKeys.counts(slug, commodityIDs),
    queryFn: ({ signal }) => getServiceCounts(commodityIDs, signal),
    enabled: enabled && commodityIDs.length > 0 && !!slug,
    placeholderData: (prev) => prev,
  })
}

function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: serviceKeys.group(slug) }),
    forCommodity: (commodityID: string) =>
      qc.invalidateQueries({ queryKey: serviceKeys.byCommodity(slug, commodityID) }),
  }
}

export function useStartService() {
  const invalidate = useInvalidate()
  return useMutation<ServiceEntity & { id: string }, Error, StartServiceRequest>({
    mutationFn: (req) => startService(req),
    onSuccess: (_svc, vars) => {
      invalidate.forCommodity(vars.commodity_id)
      invalidate.all()
    },
  })
}

interface UpdateServiceVars {
  commodityID: string
  serviceID: string
  req: UpdateServiceRequest
}

export function useUpdateService() {
  const invalidate = useInvalidate()
  return useMutation<ServiceEntity & { id: string }, Error, UpdateServiceVars>({
    mutationFn: ({ commodityID, serviceID, req }) => updateService(commodityID, serviceID, req),
    onSuccess: (_svc, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}

interface ReturnServiceVars {
  commodityID: string
  serviceID: string
  returnedAt?: string
  costAmount?: string
  costCurrency?: string
}

export function useReturnService() {
  const invalidate = useInvalidate()
  return useMutation<ServiceEntity & { id: string }, Error, ReturnServiceVars>({
    mutationFn: ({ commodityID, serviceID, returnedAt, costAmount, costCurrency }) =>
      returnService(commodityID, serviceID, { returnedAt, costAmount, costCurrency }),
    onSuccess: (_svc, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}

interface DeleteServiceVars {
  commodityID: string
  serviceID: string
}

export function useDeleteService() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, DeleteServiceVars>({
    mutationFn: ({ commodityID, serviceID }) => deleteService(commodityID, serviceID),
    onSuccess: (_void, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}
