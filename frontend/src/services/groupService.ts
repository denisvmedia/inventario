import api from './api'
import type {
  LocationGroup,
  GroupMembership,
  GroupInvite,
  InviteInfo,
  GroupCreateRequest,
  GroupUpdateRequest,
  GroupDeleteRequest,
  MemberRoleUpdateRequest,
} from '../types/group'

const API_URL = '/api/v1'

// --- Group CRUD ---

async function listGroups(): Promise<LocationGroup[]> {
  const response = await api.get(`${API_URL}/groups`)
  return response.data.data.map((item: any) => ({
    id: item.id,
    ...item.attributes,
  }))
}

async function getGroup(groupId: string): Promise<LocationGroup> {
  const response = await api.get(`${API_URL}/groups/${groupId}`)
  return {
    id: response.data.data.id,
    ...response.data.data.attributes,
  }
}

async function createGroup(data: GroupCreateRequest): Promise<LocationGroup> {
  const response = await api.post(`${API_URL}/groups`, {
    data: {
      type: 'groups',
      attributes: data,
    },
  })
  return {
    id: response.data.data.id,
    ...response.data.data.attributes,
  }
}

async function updateGroup(groupId: string, data: GroupUpdateRequest): Promise<LocationGroup> {
  const response = await api.patch(`${API_URL}/groups/${groupId}`, {
    data: {
      type: 'groups',
      id: groupId,
      attributes: data,
    },
  })
  return {
    id: response.data.data.id,
    ...response.data.data.attributes,
  }
}

async function deleteGroup(groupId: string, data: GroupDeleteRequest): Promise<void> {
  await api.delete(`${API_URL}/groups/${groupId}`, { data })
}

// --- Members ---

async function listMembers(groupId: string): Promise<GroupMembership[]> {
  const response = await api.get(`${API_URL}/groups/${groupId}/members`)
  return response.data.data.map((item: any) => ({
    id: item.id,
    ...item.attributes,
  }))
}

async function removeMember(groupId: string, userId: string): Promise<void> {
  await api.delete(`${API_URL}/groups/${groupId}/members/${userId}`)
}

async function updateMemberRole(
  groupId: string,
  userId: string,
  data: MemberRoleUpdateRequest,
): Promise<GroupMembership> {
  const response = await api.patch(`${API_URL}/groups/${groupId}/members/${userId}`, {
    data: {
      attributes: data,
    },
  })
  return {
    id: response.data.data.id,
    ...response.data.data.attributes,
  }
}

async function leaveGroup(groupId: string): Promise<void> {
  await api.post(`${API_URL}/groups/${groupId}/leave`)
}

// --- Invites ---

async function createInvite(groupId: string): Promise<GroupInvite> {
  const response = await api.post(`${API_URL}/groups/${groupId}/invites`)
  return {
    id: response.data.data.id,
    ...response.data.data.attributes,
  }
}

async function listInvites(groupId: string): Promise<GroupInvite[]> {
  const response = await api.get(`${API_URL}/groups/${groupId}/invites`)
  return response.data.data.map((item: any) => ({
    id: item.id,
    ...item.attributes,
  }))
}

async function revokeInvite(groupId: string, inviteId: string): Promise<void> {
  await api.delete(`${API_URL}/groups/${groupId}/invites/${inviteId}`)
}

async function getInviteInfo(token: string): Promise<InviteInfo> {
  const response = await api.get(`${API_URL}/invites/${token}`)
  return response.data.data.attributes
}

async function acceptInvite(token: string): Promise<GroupMembership> {
  const response = await api.post(`${API_URL}/invites/${token}/accept`)
  return {
    id: response.data.data.id,
    ...response.data.data.attributes,
  }
}

export default {
  listGroups,
  getGroup,
  createGroup,
  updateGroup,
  deleteGroup,
  listMembers,
  removeMember,
  updateMemberRole,
  leaveGroup,
  createInvite,
  listInvites,
  revokeInvite,
  getInviteInfo,
  acceptInvite,
}
