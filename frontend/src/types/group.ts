export type GroupRole = 'admin' | 'user'

export type LocationGroupStatus = 'active' | 'pending_deletion'

export interface LocationGroup {
  id: string
  slug: string
  name: string
  icon: string
  status: LocationGroupStatus
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
}

export interface GroupUpdateRequest {
  name: string
  icon?: string
}

export interface GroupDeleteRequest {
  confirm_word: string
}

export interface MemberRoleUpdateRequest {
  role: GroupRole
}
