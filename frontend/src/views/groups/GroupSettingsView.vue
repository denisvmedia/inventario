<template>
  <PageContainer width="default" class="group-settings">
    <PageHeader title="Group Settings" />

    <div v-if="loading" class="loading text-center py-8 text-muted-foreground">Loading...</div>
    <div v-else-if="!group" class="error-message text-sm text-destructive">Group not found</div>
    <template v-else>
      <!-- Group Info -->
      <section class="settings-section mb-8 rounded-md border bg-card p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold">General</h2>
        <form class="settings-form flex max-w-md flex-col gap-4" @submit.prevent="updateGroup">
          <div class="flex flex-col gap-2">
            <Label for="group-name">Name</Label>
            <Input id="group-name" v-model="editName" type="text" maxlength="100" />
          </div>
          <div class="flex flex-col gap-2">
            <Label for="group-settings-icon-trigger">Icon</Label>
            <IconPicker
              v-model="editIcon"
              trigger-id="group-settings-icon-trigger"
              trigger-label="Choose an icon"
              panel-aria-label="Pick a group icon"
              trigger-test-id="group-settings-icon-picker"
            />
          </div>
          <div class="flex flex-col gap-2">
            <Label>Slug (read-only)</Label>
            <Input :model-value="group.slug" type="text" readonly disabled />
          </div>
          <div>
            <Button type="submit" :disabled="isSaving">
              {{ isSaving ? 'Saving...' : 'Save Changes' }}
            </Button>
          </div>
        </form>
      </section>

      <!-- Main Currency (group-scoped valuation currency, read-only) -->
      <section class="settings-section mb-8 rounded-md border bg-card p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold">Main Currency</h2>
        <p class="section-hint mb-2 text-sm text-muted-foreground">
          The currency this group values its inventory in. Set once when the group
          was created and immutable after — a reprice-aware currency-migration
          tool is tracked under
          <a class="text-primary hover:underline" href="https://github.com/denisvmedia/inventario/issues/202" target="_blank" rel="noopener">#202</a>.
        </p>
        <p class="main-currency-readonly m-0 text-base"><strong>{{ group.main_currency || '—' }}</strong></p>
      </section>

      <!-- Members -->
      <section class="settings-section mb-8 rounded-md border bg-card p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold">Members ({{ members.length }})</h2>
        <div class="members-list">
          <div v-for="member in members" :key="member.id" class="member-item flex items-center justify-between border-b border-border py-2 last:border-b-0">
            <div class="member-info flex items-center gap-2">
              <span class="member-user">{{ member.member_user_id }}</span>
              <span class="member-role" :class="'role-' + member.role">{{ member.role }}</span>
            </div>
            <div v-if="isAdmin" class="member-actions flex items-center gap-2">
              <select
                :value="member.role"
                class="role-select rounded border border-input bg-background px-2 py-1 text-sm"
                @change="changeMemberRole(member.member_user_id, ($event.target as HTMLSelectElement).value as 'admin' | 'user')"
              >
                <option value="admin">Admin</option>
                <option value="user">User</option>
              </select>
              <Button
                v-if="isLastAdminMember(member)"
                variant="destructive"
                size="sm"
                disabled
                aria-disabled="true"
                aria-describedby="remove-last-admin-desc"
                :title="REMOVE_LAST_ADMIN_TOOLTIP"
                :data-testid="`remove-member-btn-${member.member_user_id}`"
              >
                Remove
              </Button>
              <Button
                v-else
                variant="destructive"
                size="sm"
                :data-testid="`remove-member-btn-${member.member_user_id}`"
                @click="removeMember(member.member_user_id)"
              >
                Remove
              </Button>
            </div>
          </div>
        </div>
        <!-- Rendered only when a disabled Remove button references it, so the
             description doesn't leak into the accessibility tree on pages
             where no such button exists (multiple admins, or non-admin
             viewers who don't see the actions block at all). -->
        <span v-if="isAdmin && adminCount === 1" id="remove-last-admin-desc" class="sr-only">{{ REMOVE_LAST_ADMIN_TOOLTIP }}</span>
      </section>

      <!-- Invites -->
      <section v-if="isAdmin" class="settings-section mb-8 rounded-md border bg-card p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold">Invite Links</h2>
        <Button :disabled="isCreatingInvite" @click="createInvite">
          {{ isCreatingInvite ? 'Generating...' : 'Generate Invite Link' }}
        </Button>
        <div v-if="newInviteUrl" class="invite-url mt-3 mb-4 flex gap-2">
          <Input :model-value="newInviteUrl" type="text" readonly class="flex-1 text-xs" />
          <Button variant="outline" @click="copyInviteUrl">Copy</Button>
        </div>
        <div class="invites-list mt-4">
          <div v-for="invite in invites" :key="invite.id" class="invite-item flex items-center justify-between border-b border-border py-2">
            <div class="invite-info">
              <code class="invite-token text-xs text-muted-foreground">{{ invite.token.substring(0, 12) }}...</code>
              <span class="invite-expires ml-4 text-xs text-muted-foreground">Expires: {{ new Date(invite.expires_at).toLocaleString() }}</span>
            </div>
            <Button variant="destructive" size="sm" @click="revokeInvite(invite.id)">Revoke</Button>
          </div>
          <p v-if="invites.length === 0" class="empty-state italic text-muted-foreground">No active invite links.</p>
        </div>
      </section>

      <!-- Leave Group -->
      <section class="settings-section mb-8 rounded-md border bg-card p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold">Leave Group</h2>
        <template v-if="isLastAdmin">
          <p class="leave-warning mb-3 rounded border-l-4 border-amber-500 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:bg-amber-950/40 dark:text-amber-200" data-testid="last-admin-notice">
            You are the last admin of this group.
            <template v-if="hasPromotableMembers">Promote another member to admin before leaving, or delete the group below.</template>
            <template v-else>To remove your access, delete the group below.</template>
          </p>
          <Button
            class="bg-amber-500 text-white hover:bg-amber-600 disabled:opacity-50"
            disabled
            aria-disabled="true"
            aria-describedby="last-admin-notice-desc"
            :title="LAST_ADMIN_TOOLTIP"
            data-testid="leave-group-btn"
          >
            Leave Group
          </Button>
          <span id="last-admin-notice-desc" class="sr-only">{{ LAST_ADMIN_TOOLTIP }}</span>
        </template>
        <template v-else>
          <p class="mb-3 text-sm text-muted-foreground">You will lose access to all data in this group.</p>
          <Button
            class="bg-amber-500 text-white hover:bg-amber-600"
            data-testid="leave-group-btn"
            @click="handleLeave"
          >
            Leave Group
          </Button>
        </template>
      </section>

      <!-- Danger Zone -->
      <section v-if="isAdmin" class="settings-section danger-zone mb-8 rounded-md border border-destructive/40 bg-destructive/5 p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold text-destructive">Danger Zone</h2>
        <p class="mb-3 text-sm">Deleting this group will permanently remove all locations, items, files, and exports. This action cannot be undone.</p>
        <Button variant="destructive" data-testid="delete-group-open" @click="openDeleteDialog">Delete Group</Button>
        <div v-if="showDeleteConfirm" class="delete-confirm mt-4 rounded-md bg-destructive/10 p-4">
          <p class="mb-2 text-sm">
            To confirm, type the group name
            <strong>{{ group.name }}</strong>
            and enter your current password. Both are required — the name
            guards against accidental clicks, the password against a hijacked
            session.
          </p>
          <Label class="mt-2 block text-xs font-semibold">Group name</Label>
          <Input
            v-model="deleteConfirmWord"
            type="text"
            class="mt-1 max-w-xs"
            data-testid="delete-confirm-word"
            :placeholder="group.name"
            autocomplete="off"
          />
          <Label class="mt-3 block text-xs font-semibold">Your password</Label>
          <Input
            v-model="deletePassword"
            type="password"
            class="mt-1 max-w-xs"
            data-testid="delete-password"
            placeholder="Current password"
            autocomplete="current-password"
          />
          <p v-if="deleteWrongPassword" class="field-error mt-1 text-sm text-destructive" data-testid="delete-password-error">
            The password you entered is incorrect.
          </p>
          <p v-if="deleteWrongConfirmWord" class="field-error mt-1 text-sm text-destructive" data-testid="delete-confirm-error">
            The group name doesn't match.
          </p>
          <div class="delete-confirm-actions mt-3 flex gap-2">
            <Button variant="outline" @click="cancelDeleteDialog">Cancel</Button>
            <Button
              variant="destructive"
              data-testid="delete-group-submit"
              :disabled="!canSubmitDelete"
              @click="handleDelete"
            >
              {{ isDeleting ? 'Deleting...' : 'Delete Permanently' }}
            </Button>
          </div>
        </div>
      </section>
    </template>

    <p v-if="error" class="error-message text-sm text-destructive">{{ error }}</p>
  </PageContainer>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import groupService from '@/services/groupService'
import IconPicker from '@/components/IconPicker.vue'
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
const deletePassword = ref('')
const deleteWrongPassword = ref(false)
const deleteWrongConfirmWord = ref(false)
const isDeleting = ref(false)

const canSubmitDelete = computed(() =>
  !isDeleting.value &&
  deleteConfirmWord.value.trim() !== '' &&
  deletePassword.value !== '',
)

function openDeleteDialog() {
  showDeleteConfirm.value = true
  deleteConfirmWord.value = ''
  deletePassword.value = ''
  deleteWrongPassword.value = false
  deleteWrongConfirmWord.value = false
}

function cancelDeleteDialog() {
  showDeleteConfirm.value = false
  deleteConfirmWord.value = ''
  deletePassword.value = ''
  deleteWrongPassword.value = false
  deleteWrongConfirmWord.value = false
}

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
      await groupStore.restoreFromPreference()
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
  deleteWrongPassword.value = false
  deleteWrongConfirmWord.value = false
  try {
    await groupService.deleteGroup(group.value.id, {
      confirm_word: deleteConfirmWord.value.trim(),
      password: deletePassword.value,
    })
    groupStore.clearCurrentGroup()
    await groupStore.fetchGroups()
    if (groupStore.hasGroups) {
      await groupStore.restoreFromPreference()
      router.push('/')
    } else {
      router.push({ name: 'no-group' })
    }
  } catch (err: any) {
    // Two distinct sentinels come back here:
    //   services.ErrInvalidPassword    → "invalid password"
    //   services.ErrInvalidConfirmation → "invalid deletion confirmation"
    // Both are marshaled by errormarshal as:
    //   errors[0] = { status: "Unprocessable Entity", error: { error: { message, sentinels: [...] }, type: "*errx.sentinel" } }
    // Read the sentinels array first (authoritative), fall back to the
    // message string, and only then to errors[0].detail for defensive
    // compatibility with other error shapes in the rest of the API.
    const apiError = err?.response?.data?.errors?.[0]
    const inner = apiError?.error?.error
    const sentinels: string[] = Array.isArray(inner?.sentinels) ? inner.sentinels : []
    const message: string = typeof inner?.message === 'string' ? inner.message : ''
    const detail: string = typeof apiError?.detail === 'string' ? apiError.detail : ''
    const probes = [...sentinels, message, detail].map((s) => s.toLowerCase())

    const matches = (needle: string) => probes.some((p) => p.includes(needle))
    if (matches('invalid password')) {
      deleteWrongPassword.value = true
      deletePassword.value = ''
    } else if (matches('invalid deletion confirmation')) {
      deleteWrongConfirmWord.value = true
    } else {
      error.value = message || detail || 'Failed to delete group'
    }
  } finally {
    isDeleting.value = false
  }
}

onMounted(loadData)
</script>

<style scoped lang="scss">
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
</style>
