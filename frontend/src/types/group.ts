export type GroupRole = 'admin' | 'user'

export type LocationGroupStatus = 'active' | 'pending_deletion'

export interface LocationGroup {
  id: string
  slug: string
  name: string
  icon: string
  status: LocationGroupStatus
  // main_currency is the ISO-4217 code the group values its inventory in.
  // Moved here (from the user-scoped /settings object) in #1248 — a user
  // who belongs to groups valued in different currencies needs to see each
  // group's currency independently.
  main_currency: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface GroupMembership {
  id: string
  group_id: string
  member_user_id: string
  role: GroupRole
  joined_at: string
}

export interface GroupInvite {
  id: string
  group_id: string
  token: string
  created_by: string
  expires_at: string
  used_by?: string
  used_at?: string
  created_at: string
}

export interface InviteInfo {
  group_name: string
  group_icon: string
  expired: boolean
  used: boolean
}

export interface GroupCreateRequest {
  name: string
  icon?: string
  // main_currency is set once at group creation and is immutable after
  // (see #202 for the currency-migration tool). Omitted → backend defaults
  // to USD.
  main_currency?: string
}

export interface GroupUpdateRequest {
  name: string
  icon?: string
}

export interface GroupDeleteRequest {
  confirm_word: string
  password: string
}

export interface MemberRoleUpdateRequest {
  role: GroupRole
}
