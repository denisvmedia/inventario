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

      <!-- Main Currency (group-scoped valuation currency) -->
      <section class="settings-section">
        <h2>Main Currency</h2>
        <p class="section-hint">
          The currency this group values its inventory in. Changing it triggers a reprice
          of every commodity in the group using the exchange rate below (falls back to a
          built-in rate table when left blank).
        </p>
        <form class="settings-form" @submit.prevent="updateMainCurrency">
          <div class="form-group">
            <label for="main-currency">Currency</label>
            <input
              id="main-currency"
              v-model="editMainCurrency"
              type="text"
              class="form-input"
              maxlength="3"
              placeholder="e.g. USD"
              :disabled="!isAdmin"
            />
          </div>
          <div v-if="isMainCurrencyChange" class="form-group">
            <label for="exchange-rate">Exchange Rate (optional)</label>
            <input
              id="exchange-rate"
              v-model="editExchangeRate"
              type="number"
              class="form-input"
              inputmode="decimal"
              min="0"
              step="any"
              placeholder="Leave blank to use the default rate"
              :disabled="!isAdmin"
            />
            <div class="field-help">
              Example: 1 {{ originalMainCurrency }} = 0.92 {{ editMainCurrency }}
            </div>
          </div>
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="!isAdmin || isSavingCurrency || !isMainCurrencyChange"
          >
            {{ isSavingCurrency ? 'Saving...' : 'Save Currency' }}
          </button>
        </form>
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
              <button class="btn btn-danger btn-small" @click="removeMember(member.member_user_id)">Remove</button>
            </div>
          </div>
        </div>
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
        <p>You will lose access to all data in this group.</p>
        <button class="btn btn-warning" @click="handleLeave">Leave Group</button>
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

const editMainCurrency = ref('')
const originalMainCurrency = ref('')
const editExchangeRate = ref<string>('')
const isSavingCurrency = ref(false)

const isMainCurrencyChange = computed(
  () => editMainCurrency.value.trim() !== '' && editMainCurrency.value.trim().toUpperCase() !== originalMainCurrency.value
)

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

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const groupId = route.params.groupId as string
    group.value = await groupService.getGroup(groupId)
    editName.value = group.value.name
    editIcon.value = group.value.icon
    editMainCurrency.value = group.value.main_currency || ''
    originalMainCurrency.value = group.value.main_currency || ''
    editExchangeRate.value = ''
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

async function updateMainCurrency() {
  if (!group.value) return
  const newCurrency = editMainCurrency.value.trim().toUpperCase()
  if (!newCurrency || newCurrency === originalMainCurrency.value) return

  isSavingCurrency.value = true
  error.value = null
  try {
    // Full group PATCH so name/icon aren't clobbered to empty strings.
    const updated = await groupService.updateGroup(group.value.id, {
      name: group.value.name,
      icon: group.value.icon,
      main_currency: newCurrency,
      exchange_rate: editExchangeRate.value.trim() || undefined,
    })
    group.value = updated
    originalMainCurrency.value = updated.main_currency
    editMainCurrency.value = updated.main_currency
    editExchangeRate.value = ''
    // Keep groupStore in sync so valuation-dependent views reload with the new currency.
    if (groupStore.currentGroup && groupStore.currentGroup.id === updated.id) {
      groupStore.setCurrentGroupById(updated.id)
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to update main currency'
  } finally {
    isSavingCurrency.value = false
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

.section-hint,
.field-help {
  color: #666;
  font-size: 0.85em;
  margin-bottom: 1em;
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
