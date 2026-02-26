<template>
  <div class="user-list">
    <div class="header">
      <h1>User Management</h1>
      <button class="btn btn-primary" @click="openCreateForm">
        <font-awesome-icon icon="plus" /> New User
      </button>
    </div>

    <!-- Filters -->
    <div class="filters">
      <input
        v-model="search"
        type="text"
        class="filter-input"
        placeholder="Search by name or email…"
        @input="onFilterChange"
      />
      <select v-model="roleFilter" class="filter-select" @change="onFilterChange">
        <option value="">All roles</option>
        <option value="admin">Admin</option>
        <option value="user">User</option>
      </select>
      <select v-model="activeFilter" class="filter-select" @change="onFilterChange">
        <option value="">All statuses</option>
        <option value="true">Active</option>
        <option value="false">Inactive</option>
      </select>
    </div>

    <!-- Error Notification Stack -->
    <ErrorNotificationStack :errors="errors" @dismiss="removeError" />

    <div v-if="loading" class="loading">Loading…</div>
    <div v-else-if="users.length === 0 && !loading" class="empty">
      <p>No users found.</p>
    </div>
    <div v-else class="table-wrapper">
      <table class="users-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Role</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td>{{ user.name }}</td>
            <td>{{ user.email }}</td>
            <td>
              <span :class="['badge', user.role === 'admin' ? 'badge-admin' : 'badge-user']">
                {{ user.role }}
              </span>
            </td>
            <td>
              <span :class="['badge', user.is_active ? 'badge-active' : 'badge-inactive']">
                {{ user.is_active ? 'Active' : 'Inactive' }}
              </span>
            </td>
            <td class="actions">
              <button class="btn btn-secondary btn-sm" title="Edit" @click="openEditForm(user)">
                <font-awesome-icon icon="edit" />
              </button>
              <button
                v-if="user.is_active && user.id !== currentUserId"
                class="btn btn-danger btn-sm"
                title="Deactivate"
                @click="confirmDeactivate(user)"
              >
                <font-awesome-icon icon="ban" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="pagination">
      <button :disabled="page <= 1" class="btn btn-secondary btn-sm" @click="goToPage(page - 1)">
        <font-awesome-icon icon="chevron-left" />
      </button>
      <span class="page-info">Page {{ page }} of {{ totalPages }} ({{ total }} users)</span>
      <button :disabled="page >= totalPages" class="btn btn-secondary btn-sm" @click="goToPage(page + 1)">
        <font-awesome-icon icon="chevron-right" />
      </button>
    </div>
    <div v-else-if="!loading && total > 0" class="pagination-info">
      {{ total }} user{{ total !== 1 ? 's' : '' }}
    </div>

    <!-- Create / Edit Dialog -->
    <div v-if="showForm" class="modal-overlay" @click.self="closeForm">
      <div class="modal">
        <div class="modal-header">
          <h2>{{ editingUser ? 'Edit User' : 'Create User' }}</h2>
          <button class="btn-close" @click="closeForm"><font-awesome-icon icon="times" /></button>
        </div>
        <div class="modal-body">
          <UserForm
            :user="editingUser"
            :saving="saving"
            @save="onSave"
            @cancel="closeForm"
          />
        </div>
      </div>
    </div>

    <!-- Deactivate Confirmation -->
    <Confirmation
      v-model:visible="showDeactivateDialog"
      title="Deactivate User"
      :message="`Are you sure you want to deactivate ${deactivatingUser?.name}?`"
      confirm-label="Deactivate"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmationIcon="exclamation-triangle"
      @confirm="onConfirmDeactivate"
      @cancel="showDeactivateDialog = false"
    />
  </div>
</template>



<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '@/stores/authStore'
import userService from '@/services/userService'
import { useErrorState } from '@/utils/errorUtils'
import Confirmation from '@/components/Confirmation.vue'
import ErrorNotificationStack from '@/components/ErrorNotificationStack.vue'
import UserForm from '@/views/admin/UserForm.vue'
import type { AdminUser } from '@/types'

const authStore = useAuthStore()
const { errors, handleError, removeError } = useErrorState()

const currentUserId = computed(() => authStore.user?.id ?? '')

const users = ref<AdminUser[]>([])
const loading = ref(false)
const saving = ref(false)
const total = ref(0)
const page = ref(1)
const totalPages = ref(1)
const perPage = 20

// Filters
const search = ref('')
const roleFilter = ref('')
const activeFilter = ref('')

let filterDebounce: ReturnType<typeof setTimeout> | null = null

async function loadUsers() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: page.value, per_page: perPage }
    if (search.value) params.search = search.value
    if (roleFilter.value) params.role = roleFilter.value
    if (activeFilter.value !== '') params.active = activeFilter.value === 'true'

    const data = await userService.listUsers(params)
    users.value = data.users ?? []
    total.value = data.total
    totalPages.value = data.total_pages
  } catch (err: any) {
    handleError(err, 'user', 'Failed to load users')
  } finally {
    loading.value = false
  }
}

function onFilterChange() {
  if (filterDebounce) clearTimeout(filterDebounce)
  filterDebounce = setTimeout(() => {
    page.value = 1
    loadUsers()
  }, 300)
}

function goToPage(p: number) {
  page.value = p
  loadUsers()
}

// Create / Edit form
const showForm = ref(false)
const editingUser = ref<AdminUser | null>(null)

function openCreateForm() {
  editingUser.value = null
  showForm.value = true
}

function openEditForm(user: AdminUser) {
  editingUser.value = user
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingUser.value = null
}

async function onSave(data: any) {
  saving.value = true
  try {
    if (editingUser.value) {
      const updated = await userService.updateUser(editingUser.value.id, data)
      const idx = users.value.findIndex(u => u.id === updated.id)
      if (idx !== -1) users.value[idx] = updated
    } else {
      await userService.createUser(data)
      await loadUsers()
    }
    closeForm()
  } catch (err: any) {
    handleError(err, 'user', editingUser.value ? 'Failed to update user' : 'Failed to create user')
  } finally {
    saving.value = false
  }
}

// Deactivation
const showDeactivateDialog = ref(false)
const deactivatingUser = ref<AdminUser | null>(null)

function confirmDeactivate(user: AdminUser) {
  deactivatingUser.value = user
  showDeactivateDialog.value = true
}

async function onConfirmDeactivate() {
  showDeactivateDialog.value = false
  if (!deactivatingUser.value) return
  try {
    await userService.deactivateUser(deactivatingUser.value.id)
    const idx = users.value.findIndex(u => u.id === deactivatingUser.value!.id)
    if (idx !== -1) users.value[idx] = { ...users.value[idx], is_active: false }
  } catch (err: any) {
    handleError(err, 'user', 'Failed to deactivate user')
  } finally {
    deactivatingUser.value = null
  }
}

onMounted(loadUsers)
</script>


<style scoped>
.user-list {
  padding: 1.5rem;
  max-width: 1200px;
  margin: 0 auto;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1.25rem;
}
.header h1 {
  margin: 0;
  font-size: 1.5rem;
}
.filters {
  display: flex;
  gap: 0.75rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}
.filter-input,
.filter-select {
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--p-surface-300, #ccc);
  border-radius: 6px;
  font-size: 0.9rem;
  background: var(--p-surface-0, #fff);
  color: var(--p-text-color, inherit);
}
.filter-input {
  flex: 1;
  min-width: 200px;
}
.loading,
.empty {
  text-align: center;
  padding: 3rem;
  color: var(--p-text-muted-color, #888);
}
.table-wrapper {
  overflow-x: auto;
}
.users-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}
.users-table th,
.users-table td {
  padding: 0.75rem 1rem;
  text-align: left;
  border-bottom: 1px solid var(--p-surface-200, #e5e7eb);
}
.users-table th {
  font-weight: 600;
  background: var(--p-surface-100, #f9fafb);
}
.users-table tr:hover td {
  background: var(--p-surface-50, #f3f4f6);
}
.actions {
  display: flex;
  gap: 0.5rem;
}
.badge {
  display: inline-block;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: capitalize;
}
.badge-admin { background: #dbeafe; color: #1d4ed8; }
.badge-user  { background: #f3f4f6; color: #374151; }
.badge-active   { background: #dcfce7; color: #166534; }
.badge-inactive { background: #fee2e2; color: #991b1b; }
.pagination {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 1rem;
  justify-content: center;
}
.pagination-info {
  margin-top: 1rem;
  text-align: center;
  color: var(--p-text-muted-color, #888);
  font-size: 0.85rem;
}
.page-info {
  font-size: 0.9rem;
  color: var(--p-text-muted-color, #555);
}
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.modal {
  background: var(--p-surface-0, #fff);
  border-radius: 10px;
  min-width: 420px;
  max-width: 560px;
  width: 100%;
  box-shadow: 0 20px 60px rgba(0,0,0,0.25);
}
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 1.5rem 1rem;
  border-bottom: 1px solid var(--p-surface-200, #e5e7eb);
}
.modal-header h2 {
  margin: 0;
  font-size: 1.15rem;
}
.modal-body {
  padding: 1.25rem 1.5rem 1.5rem;
}
.btn-close {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  color: var(--p-text-muted-color, #888);
}
</style>
