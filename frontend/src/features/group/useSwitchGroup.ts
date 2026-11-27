import { useEffect, useRef } from "react"
import { useNavigate, useParams } from "react-router-dom"

import { useOptionalAuth } from "@/features/auth/AuthContext"
import { useUpdateProfile } from "@/features/auth/hooks"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"

const SAVE_DELAY_MS = 400

interface SwitchTarget {
  id?: string
  slug?: string
}

// useSwitchGroup returns the handler every group switcher calls, so the
// sidebar selector and the top bar switcher do the same thing. Switching
// opens /g/<slug> and saves the group as the user's default_group_id, which
// decides the group a fresh sign-in lands in on any device.
//
// The save waits SAVE_DELAY_MS: a run of quick switches sends one
// PUT /auth/me with the last group. A save still waiting when the switcher
// unmounts (the mobile sidebar sheet closes on navigation) is sent then.
export function useSwitchGroup(): (group: SwitchTarget | undefined) => void {
  const navigate = useNavigate()
  const params = useParams<{ groupSlug?: string }>()
  const groupCtx = useOptionalCurrentGroup()
  const auth = useOptionalAuth()
  const updateProfile = useUpdateProfile()

  const activeSlug = groupCtx?.currentGroup?.slug ?? params.groupSlug ?? null
  const pending = useRef<string | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  // The mutate function of the latest render, for the unmount flush.
  const mutateRef = useRef(updateProfile.mutate)
  useEffect(() => {
    mutateRef.current = updateProfile.mutate
  })

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
      const next = pending.current
      pending.current = null
      if (next) mutateRef.current({ default_group_id: next })
    },
    []
  )

  return (group) => {
    if (!group?.slug || group.slug === activeSlug) return
    navigate(`/g/${encodeURIComponent(group.slug)}`)
    if (!group.id || !auth?.user) return
    // Only the group is sent: the name this page loaded may be stale, and
    // the server keeps the stored name when the body has none.
    pending.current = group.id
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => {
      timer.current = null
      const next = pending.current
      pending.current = null
      if (next) mutateRef.current({ default_group_id: next })
    }, SAVE_DELAY_MS)
  }
}
