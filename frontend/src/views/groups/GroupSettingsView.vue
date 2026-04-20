<template>
  <div class="group-settings container">
    <h1>Group Settings</h1>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="!group" class="error-message">Group not found</div>
    <template v-else>
      <!-- Group Info -->
      <section class="settings-section">
        <h2>General</h2>
        <form class="settings-form" @submit.prevent="updateGroup">
          <div class="form-group">
            <label for="group-name">Name</label>
            <input id="group-name" v-model="editName" type="text" class="form-input" maxlength="100" />
          </div>
          <div class="form-group">
            <label for="group-icon">Icon</label>
            <input id="group-icon" v-model="editIcon" type="text" class="form-input" maxlength="10" placeholder="e.g. 📦" />
          </div>
          <div class="form-group">
            <label>Slug (read-only)</label>
            <input :value="group.slug" type="text" class="form-input" readonly disabled />
          </div>
          <button type="submit" class="btn btn-primary" :disabled="isSaving">
            {{ isSaving ? 'Saving...' : 'Save Changes' }}
          </button>
        </form>
      </section>

      <!-- Main Currency (group-scoped valuation currency, read-only) -->
      <section class="settings-section">
        <h2>Main Currency</h2>
        <p class="section-hint">
          The currency this group values its inventory in. Set once when the group
          was created and immutable after — a reprice-aware currency-migration
          tool is tracked under
          <a href="https://github.com/denisvmedia/inventario/issues/202" target="_blank" rel="noopener">#202</a>.
        </p>
        <p class="main-currency-readonly"><strong>{{ group.main_currency || '—' }}</strong></p>
      </section>

      <!-- Members -->
      <section class="settings-section">
        <h2>Members ({{ members.length }})</h2>
        <div class="members-list">
          <div v-for="member in members" :key="member.id" class="member-item">
            <div class="member-info">
              <span class="member-user">{{ member.member_user_id }}</span>
              <span class="member-role" :class="'role-' + member.role">{{ member.role }}</span>
            </div>
            <div v-if="isAdmin" class="member-actions">
              <select
                :value="member.role"
                class="role-select"
                @change="changeMemberRole(member.member_user_id, ($event.target as HTMLSelectElement).value as 'admin' | 'user')"
              >
                <option value="admin">Admin</option>
                <option value="user">User</option>
              </select>
              <button
                v-if="isLastAdminMember(member)"
                class="btn btn-danger btn-small"
                disabled
                aria-disabled="true"
                aria-describedby="remove-last-admin-desc"
                :title="REMOVE_LAST_ADMIN_TOOLTIP"
                :data-testid="`remove-member-btn-${member.member_user_id}`"
              >
                Remove
              </button>
              <button
                v-else
                class="btn btn-danger btn-small"
                :data-testid="`remove-member-btn-${member.member_user_id}`"
                @click="removeMember(member.member_user_id)"
              >
                Remove
              </button>
            </div>
          </div>
        </div>
        <span id="remove-last-admin-desc" class="sr-only">{{ REMOVE_LAST_ADMIN_TOOLTIP }}</span>
      </section>

      <!-- Invites -->
      <section v-if="isAdmin" class="settings-section">
        <h2>Invite Links</h2>
        <button class="btn btn-primary" :disabled="isCreatingInvite" @click="createInvite">
          {{ isCreatingInvite ? 'Generating...' : 'Generate Invite Link' }}
        </button>
        <div v-if="newInviteUrl" class="invite-url">
          <input :value="newInviteUrl" type="text" readonly class="form-input" />
          <button class="btn btn-secondary" @click="copyInviteUrl">Copy</button>
        </div>
        <div class="invites-list">
          <div v-for="invite in invites" :key="invite.id" class="invite-item">
            <div class="invite-info">
              <code class="invite-token">{{ invite.token.substring(0, 12) }}...</code>
              <span class="invite-expires">Expires: {{ new Date(invite.expires_at).toLocaleString() }}</span>
            </div>
            <button class="btn btn-danger btn-small" @click="revokeInvite(invite.id)">Revoke</button>
          </div>
          <p v-if="invites.length === 0" class="empty-state">No active invite links.</p>
        </div>
      </section>

      <!-- Leave Group -->
      <section class="settings-section">
        <h2>Leave Group</h2>
        <template v-if="isLastAdmin">
          <p class="leave-warning" data-testid="last-admin-notice">
            You are the last admin of this group.
            <template v-if="hasPromotableMembers">Promote another member to admin before leaving, or delete the group below.</template>
            <template v-else>To remove your access, delete the group below.</template>
          </p>
          <button
            class="btn btn-warning"
            disabled
            aria-disabled="true"
            aria-describedby="last-admin-notice-desc"
            :title="LAST_ADMIN_TOOLTIP"
            data-testid="leave-group-btn"
          >
            Leave Group
          </button>
          <span id="last-admin-notice-desc" class="sr-only">{{ LAST_ADMIN_TOOLTIP }}</span>
        </template>
        <template v-else>
          <p>You will lose access to all data in this group.</p>
          <button class="btn btn-warning" data-testid="leave-group-btn" @click="handleLeave">Leave Group</button>
        </template>
      </section>

      <!-- Danger Zone -->
      <section v-if="isAdmin" class="settings-section danger-zone">
        <h2>Danger Zone</h2>
        <p>Deleting this group will permanently remove all locations, items, files, and exports. This action cannot be undone.</p>
        <button class="btn btn-danger" @click="showDeleteConfirm = true">Delete Group</button>
        <div v-if="showDeleteConfirm" class="delete-confirm">
          <p>To confirm, type the group name: <strong>{{ group.name }}</strong></p>
          <input v-model="deleteConfirmWord" type="text" class="form-input" :placeholder="group.name" />
          <div class="delete-confirm-actions">
            <button class="btn btn-secondary" @click="showDeleteConfirm = false">Cancel</button>
            <button
              class="btn btn-danger"
              :disabled="deleteConfirmWord !== group.name || isDeleting"
              @click="handleDelete"
            >
              {{ isDeleting ? 'Deleting...' : 'Delete Permanently' }}
            </button>
          </div>
        </div>
      </section>
    </template>

    <p v-if="error" class="error-message">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import groupService from '@/services/groupService'
import type { LocationGroup, GroupMembership, GroupInvite } from '@/types/group'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const groupStore = useGroupStore()

const group = ref<LocationGroup | null>(null)
const members = ref<GroupMembership[]>([])
const invites = ref<GroupInvite[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const editName = ref('')
const editIcon = ref('')
const isSaving = ref(false)

const newInviteUrl = ref<string | null>(null)
const isCreatingInvite = ref(false)

const showDeleteConfirm = ref(false)
const deleteConfirmWord = ref('')
const isDeleting = ref(false)

const isAdmin = computed(() => {
  const userId = authStore.user?.id
  if (!userId) return false
  return members.value.some((m) => m.member_user_id === userId && m.role === 'admin')
})

const adminCount = computed(() => members.value.filter((m) => m.role === 'admin').length)

// isLastAdmin guards the "Leave Group" button: a sole admin leaving would
// leave the group without any admin, violating the backend's ≥1-admin
// invariant. The backend also rejects the request (422 ErrLastAdmin) — this
// check is strictly about UX. Frame it in the caller's role first (isAdmin):
// a non-admin can always leave, even if they're the only non-admin member,
// so the admin-count check must not kick in for them.
const isLastAdmin = computed(() => isAdmin.value && adminCount.value === 1)

// Whether there is a non-admin member the last admin could promote as a
// prerequisite to leaving. When false, "delete the group" is the only
// sensible suggestion — avoid telling the user to promote nobody.
const hasPromotableMembers = computed(() => members.value.some((m) => m.role === 'user'))

const LAST_ADMIN_TOOLTIP = 'You are the last admin. Promote another member first, or delete the group.'

// Mirror of the backend ≥1-admin invariant for the member list: if the target
// is an admin AND there is only one admin, removing them would leave the
// group unmanageable. The backend rejects such requests with 422
// ErrLastAdmin — this predicate is the UI-side gate so the Remove button is
// disabled up-front instead of only after a failed round-trip.
function isLastAdminMember(member: GroupMembership): boolean {
  return member.role === 'admin' && adminCount.value === 1
}

const REMOVE_LAST_ADMIN_TOOLTIP =
  'Cannot remove the last admin — promote another member first or delete the group.'

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const groupId = route.params.groupId as string
    group.value = await groupService.getGroup(groupId)
    editName.value = group.value.name
    editIcon.value = group.value.icon
    members.value = await groupService.listMembers(groupId)
    if (isAdmin.value) {
      invites.value = await groupService.listInvites(groupId)
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load group settings'
  } finally {
    loading.value = false
  }
}

async function updateGroup() {
  if (!group.value) return
  isSaving.value = true
  try {
    // groupStore.updateGroupById centralizes "call service + sync local store"
    // so the component doesn't need to mutate groupStore.currentGroup / groups[].
    group.value = await groupStore.updateGroupById(group.value.id, editName.value, editIcon.value)
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to update group'
  } finally {
    isSaving.value = false
  }
}

async function changeMemberRole(userId: string, newRole: 'admin' | 'user') {
  if (!group.value) return
  try {
    await groupService.updateMemberRole(group.value.id, userId, { role: newRole })
    members.value = await groupService.listMembers(group.value.id)
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to change role'
  }
}

async function removeMember(userId: string) {
  if (!group.value || !confirm('Remove this member from the group?')) return
  try {
    await groupService.removeMember(group.value.id, userId)
    members.value = await groupService.listMembers(group.value.id)
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to remove member'
  }
}

async function createInvite() {
  if (!group.value) return
  isCreatingInvite.value = true
  try {
    const invite = await groupService.createInvite(group.value.id)
    newInviteUrl.value = `${window.location.origin}/invite/${invite.token}`
    invites.value = await groupService.listInvites(group.value.id)
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to create invite'
  } finally {
    isCreatingInvite.value = false
  }
}

async function copyInviteUrl() {
  if (newInviteUrl.value) {
    try {
      await navigator.clipboard.writeText(newInviteUrl.value)
    } catch {
      error.value = 'Failed to copy to clipboard'
    }
  }
}

async function revokeInvite(inviteId: string) {
  if (!group.value || !confirm('Revoke this invite link?')) return
  try {
    await groupService.revokeInvite(group.value.id, inviteId)
    invites.value = await groupService.listInvites(group.value.id)
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to revoke invite'
  }
}

async function handleLeave() {
  if (!group.value || !confirm('Are you sure you want to leave this group?')) return
  try {
    await groupService.leaveGroup(group.value.id)
    groupStore.clearCurrentGroup()
    await groupStore.fetchGroups()
    if (groupStore.hasGroups) {
      await groupStore.restoreFromStorage()
      router.push('/')
    } else {
      router.push({ name: 'no-group' })
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to leave group'
  }
}

async function handleDelete() {
  if (!group.value) return
  isDeleting.value = true
  try {
    await groupService.deleteGroup(group.value.id, { confirm_word: deleteConfirmWord.value })
    groupStore.clearCurrentGroup()
    await groupStore.fetchGroups()
    if (groupStore.hasGroups) {
      await groupStore.restoreFromStorage()
      router.push('/')
    } else {
      router.push({ name: 'no-group' })
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to delete group'
  } finally {
    isDeleting.value = false
  }
}

onMounted(loadData)
</script>

<style scoped lang="scss">
.settings-section {
  margin-bottom: 2em;
  padding: 1.5em;
  background: white;
  border-radius: 8px;
  border: 1px solid #eee;

  h2 {
    margin-top: 0;
    margin-bottom: 1em;
    font-size: 1.2em;
  }
}

.invite-url {
  display: flex;
  gap: 0.5em;
  margin-top: 0.8em;
  margin-bottom: 1em;

  .form-input {
    flex: 1;
    padding: 0.5em;
    border: 1px solid #ccc;
    border-radius: 6px;
    font-size: 0.85em;
  }
}

.settings-form {
  max-width: 400px;

  .form-group {
    margin-bottom: 1em;

    label {
      display: block;
      margin-bottom: 0.3em;
      font-weight: 500;
      font-size: 0.9em;
    }

    .form-input {
      width: 100%;
      padding: 0.5em;
      border: 1px solid #ccc;
      border-radius: 6px;
    }
  }
}

.section-hint {
  color: #666;
  font-size: 0.85em;
  margin-bottom: 1em;

  a {
    color: #1a73e8;
    text-decoration: none;
    &:hover { text-decoration: underline; }
  }
}

.main-currency-readonly {
  font-size: 1.1em;
  margin: 0;
}

.members-list {
  .member-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.6em 0;
    border-bottom: 1px solid #f0f0f0;

    &:last-child {
      border-bottom: none;
    }
  }

  .member-info {
    display: flex;
    align-items: center;
    gap: 0.5em;
  }

  .member-role {
    font-size: 0.8em;
    padding: 0.15em 0.5em;
    border-radius: 4px;
    font-weight: 500;

    &.role-admin {
      background: #e8f0fe;
      color: #1a73e8;
    }

    &.role-user {
      background: #f0f0f0;
      color: #666;
    }
  }

  .member-actions {
    display: flex;
    gap: 0.5em;
    align-items: center;
  }

  .role-select {
    padding: 0.25em 0.5em;
    border: 1px solid #ccc;
    border-radius: 4px;
    font-size: 0.9em;
  }
}

.invites-list {
  margin-top: 1em;

  .invite-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5em 0;
    border-bottom: 1px solid #f0f0f0;
  }

  .invite-token {
    font-size: 0.85em;
    color: #666;
  }

  .invite-expires {
    font-size: 0.8em;
    color: #999;
    margin-left: 1em;
  }

  .empty-state {
    color: #999;
    font-style: italic;
  }
}

.danger-zone {
  border-color: #fcc;
  background: #fff8f8;

  h2 {
    color: #c00;
  }

  .delete-confirm {
    margin-top: 1em;
    padding: 1em;
    background: #fff0f0;
    border-radius: 6px;

    .form-input {
      margin: 0.5em 0;
      width: 100%;
      max-width: 300px;
      padding: 0.5em;
      border: 1px solid #ccc;
      border-radius: 6px;
    }
  }

  .delete-confirm-actions {
    display: flex;
    gap: 0.5em;
    margin-top: 0.5em;
  }
}

// .btn and its -primary/-secondary/-warning/-danger/-small modifiers come
// from shared _components.scss.

.leave-warning {
  color: #8a5a00;
  background: #fff8e1;
  padding: 0.6em 0.8em;
  border-left: 3px solid #f5a623;
  border-radius: 4px;
  margin-bottom: 0.8em;
}

// Visually hidden, still available to screen readers. Local copy — the app
// doesn't expose a global .sr-only utility. Uses clip-path (not the
// deprecated clip) per current a11y guidance.
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
  border: 0;
}

.error-message {
  color: #c00;
  margin-top: 0.5em;
}

.loading {
  text-align: center;
  padding: 2em;
  color: #888;
}
</style>
