import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  createMaintenanceSchedule,
  deleteMaintenanceSchedule,
  listGroupMaintenance,
  listSchedulesForCommodity,
  markMaintenanceDone,
  updateMaintenanceSchedule,
  type CreateMaintenanceScheduleRequest,
  type ListGroupMaintenanceOptions,
  type ListedMaintenanceSchedule,
  type MaintenanceScheduleEntity,
  type UpdateMaintenanceScheduleRequest,
} from "./api"
import { maintenanceKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useMaintenanceForCommodity(
  commodityID: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ schedules: Array<MaintenanceScheduleEntity & { id: string }>; total: number }>({
    queryKey: maintenanceKeys.byCommodity(slug, commodityID ?? ""),
    queryFn: ({ signal }) => {
      if (!commodityID) {
        throw new Error("useMaintenanceForCommodity called without a commodity id")
      }
      return listSchedulesForCommodity(commodityID, signal)
    },
    enabled: enabled && !!commodityID,
  })
}

export function useGroupMaintenance(
  opts: ListGroupMaintenanceOptions = {},
  query: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ schedules: ListedMaintenanceSchedule[]; total: number }>({
    queryKey: maintenanceKeys.groupList(slug, opts),
    queryFn: ({ signal }) => listGroupMaintenance({ ...opts, signal }),
    enabled: query.enabled ?? true,
    placeholderData: (prev) => prev,
  })
}

function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: maintenanceKeys.group(slug) }),
    forCommodity: (commodityID: string) =>
      qc.invalidateQueries({ queryKey: maintenanceKeys.byCommodity(slug, commodityID) }),
  }
}

export function useCreateMaintenanceSchedule() {
  const invalidate = useInvalidate()
  return useMutation<
    MaintenanceScheduleEntity & { id: string },
    Error,
    CreateMaintenanceScheduleRequest
  >({
    mutationFn: (req) => createMaintenanceSchedule(req),
    onSuccess: (_schedule, vars) => {
      invalidate.forCommodity(vars.commodity_id)
      invalidate.all()
    },
  })
}

interface UpdateMaintenanceVars {
  commodityID: string
  scheduleID: string
  req: UpdateMaintenanceScheduleRequest
}

export function useUpdateMaintenanceSchedule() {
  const invalidate = useInvalidate()
  return useMutation<MaintenanceScheduleEntity & { id: string }, Error, UpdateMaintenanceVars>({
    mutationFn: ({ scheduleID, req }) => updateMaintenanceSchedule(scheduleID, req),
    onSuccess: (_schedule, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}

interface MarkDoneVars {
  commodityID: string
  scheduleID: string
  doneAt?: string
}

export function useMarkMaintenanceDone() {
  const invalidate = useInvalidate()
  return useMutation<MaintenanceScheduleEntity & { id: string }, Error, MarkDoneVars>({
    mutationFn: ({ scheduleID, doneAt }) => markMaintenanceDone(scheduleID, doneAt),
    onSuccess: (_schedule, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}

interface DeleteMaintenanceVars {
  commodityID: string
  scheduleID: string
}

export function useDeleteMaintenanceSchedule() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, DeleteMaintenanceVars>({
    mutationFn: ({ scheduleID }) => deleteMaintenanceSchedule(scheduleID),
    onSuccess: (_void, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}
