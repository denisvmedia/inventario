<template>
  <div class="invite-accept">
    <div class="invite-card">
      <div v-if="loading" class="invite-loading">Loading invite...</div>

      <div v-else-if="error" class="invite-error">
        <h2>Invalid Invite</h2>
        <p>{{ error }}</p>
        <router-link to="/" class="btn btn-primary">Go to Home</router-link>
      </div>

      <template v-else-if="inviteInfo">
        <div class="invite-header">
          <span v-if="inviteInfo.group_icon" class="invite-icon">{{ inviteInfo.group_icon }}</span>
          <h2>Join {{ inviteInfo.group_name }}</h2>
        </div>

        <div v-if="inviteInfo.expired" class="invite-status invite-status--expired">
          This invite link has expired. Ask the group admin to generate a new one.
        </div>

        <div v-else-if="inviteInfo.used" class="invite-status invite-status--used">
          This invite link has already been used.
        </div>

        <template v-else>
          <div v-if="authStore.isAuthenticated" class="invite-action">
            <p>You've been invited to join <strong>{{ inviteInfo.group_name }}</strong>.</p>
            <button class="btn btn-primary" @click="acceptInvite" :disabled="isAccepting">
              {{ isAccepting ? 'Joining...' : 'Join Group' }}
            </button>
            <p v-if="acceptError" class="invite-error-text">{{ acceptError }}</p>
          </div>

          <div v-else class="invite-auth">
            <p>You've been invited to join <strong>{{ inviteInfo.group_name }}</strong>.</p>
            <p>Log in or register to accept this invitation.</p>
            <div class="invite-auth-buttons">
              <router-link :to="{ path: '/login', query: { redirect: $route.fullPath } }" class="btn btn-primary">
                Log In
              </router-link>
              <router-link :to="{ path: '/register', query: { redirect: $route.fullPath } }" class="btn btn-secondary">
                Register
              </router-link>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import groupService from '@/services/groupService'
import type { InviteInfo } from '@/types/group'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const groupStore = useGroupStore()

const inviteInfo = ref<InviteInfo | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const isAccepting = ref(false)
const acceptError = ref<string | null>(null)

const token = computed(() => route.params.token as string)

async function loadInviteInfo() {
  loading.value = true
  error.value = null
  try {
    inviteInfo.value = await groupService.getInviteInfo(token.value)
  } catch {
    error.value = 'This invite link is not valid.'
  } finally {
    loading.value = false
  }
}

async function acceptInvite() {
  isAccepting.value = true
  acceptError.value = null
  try {
    const membership = await groupService.acceptInvite(token.value)
    // Clean up the pending invite token from sessionStorage
    sessionStorage.removeItem('pendingInviteToken')
    // Refresh group list and switch to the joined group
    await groupStore.fetchGroups()
    const joinedGroup = groupStore.groups.find((g) => g.id === membership.group_id)
    if (joinedGroup) {
      await groupStore.setCurrentGroup(joinedGroup.slug)
    }
    router.push('/')
  } catch (err: any) {
    acceptError.value = err.response?.data?.errors?.[0]?.detail || 'Failed to accept invite'
  } finally {
    isAccepting.value = false
  }
}

// Watch for token changes (e.g. navigating between invite links)
watch(() => route.params.token, () => {
  loadInviteInfo()
})

onMounted(loadInviteInfo)
</script>

<style scoped lang="scss">
.invite-accept {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}

.invite-card {
  text-align: center;
  max-width: 480px;
  padding: 2.5em;
  background: white;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.invite-header {
  margin-bottom: 1.5em;

  .invite-icon {
    display: block;
    font-size: 3em;
    margin-bottom: 0.3em;
  }

  h2 {
    margin: 0;
  }
}

.invite-status {
  padding: 1em;
  border-radius: 8px;
  margin-top: 1em;

  &--expired {
    background: #fff3cd;
    color: #856404;
  }

  &--used {
    background: #f0f0f0;
    color: #666;
  }
}

.invite-action {
  margin-top: 1em;

  p { margin-bottom: 1em; }
}

.invite-auth {
  margin-top: 1em;

  p { margin-bottom: 0.5em; color: #555; }
}

.invite-auth-buttons {
  display: flex;
  gap: 0.8em;
  justify-content: center;
  margin-top: 1.5em;
}

.invite-error-text {
  color: #c00;
  font-size: 0.9em;
  margin-top: 0.5em;
}

.invite-loading {
  color: #888;
  padding: 2em;
}

.invite-error {
  p { color: #666; margin: 1em 0; }
}

.btn {
  padding: 0.6em 1.5em;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  text-decoration: none;
  display: inline-block;
  font-size: 0.95em;

  &-primary { background: #4a90d9; color: white; &:hover { background: #3a7bc8; } }
  &-secondary { background: #eee; color: #333; &:hover { background: #ddd; } }
  &:disabled { opacity: 0.6; cursor: not-allowed; }
}
</style>
