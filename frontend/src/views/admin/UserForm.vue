<template>
  <form class="user-form" @submit.prevent="submit">
    <div class="field">
      <label for="name">Name <span class="required">*</span></label>
      <input id="name" v-model="form.name" type="text" required placeholder="Full name" />
    </div>
    <div class="field">
      <label for="email">Email <span class="required">*</span></label>
      <input id="email" v-model="form.email" type="email" required placeholder="user@example.com" />
    </div>
    <div class="field">
      <label for="role">Role <span class="required">*</span></label>
      <select id="role" v-model="form.role" required>
        <option value="user">User</option>
        <option value="admin">Admin</option>
      </select>
    </div>
    <div class="field">
      <label for="password">{{ user ? 'New Password (leave blank to keep)' : 'Password' }} <span v-if="!user" class="required">*</span></label>
      <input
        id="password"
        v-model="form.password"
        type="password"
        :required="!user"
        placeholder="••••••••"
        autocomplete="new-password"
      />
    </div>
    <div class="field field-inline">
      <label for="is-active">Active</label>
      <input id="is-active" v-model="form.is_active" type="checkbox" />
    </div>
    <div class="form-actions">
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="$emit('cancel')">Cancel</button>
      <button type="submit" class="btn btn-primary" :disabled="saving">
        <font-awesome-icon v-if="saving" icon="spinner" spin />
        {{ user ? 'Save Changes' : 'Create User' }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { AdminUser, AdminUserCreateRequest, AdminUserUpdateRequest } from '@/types'

const props = defineProps<{
  user?: AdminUser | null
  saving?: boolean
}>()

const emit = defineEmits<{
  (_e: 'save', _data: AdminUserCreateRequest | AdminUserUpdateRequest): void
  (_e: 'cancel'): void
}>()

const form = reactive({
  name: '',
  email: '',
  role: 'user' as 'admin' | 'user',
  password: '',
  is_active: true,
})

// Populate form when editing an existing user.
watch(
  () => props.user,
  (u) => {
    if (u) {
      form.name = u.name
      form.email = u.email
      form.role = u.role
      form.is_active = u.is_active
      form.password = ''
    } else {
      form.name = ''
      form.email = ''
      form.role = 'user'
      form.password = ''
      form.is_active = true
    }
  },
  { immediate: true },
)

function submit() {
  if (props.user) {
    // Build update payload with only provided fields.
    const data: AdminUserUpdateRequest = {
      name: form.name,
      email: form.email,
      role: form.role,
      is_active: form.is_active,
    }
    if (form.password) data.password = form.password
    emit('save', data)
  } else {
    const data: AdminUserCreateRequest = {
      name: form.name,
      email: form.email,
      role: form.role,
      password: form.password,
      is_active: form.is_active,
    }
    emit('save', data)
  }
}
</script>

<style scoped>
.user-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.field-inline {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
}

label {
  font-weight: 500;
  font-size: 0.9rem;
}

.required {
  color: #e53e3e;
}

input[type='text'],
input[type='email'],
input[type='password'],
select {
  padding: 0.45rem 0.75rem;
  border: 1px solid var(--p-surface-300, #ccc);
  border-radius: 6px;
  font-size: 0.9rem;
  background: var(--p-surface-0, #fff);
  color: var(--p-text-color, inherit);
}

input[type='text']:focus,
input[type='email']:focus,
input[type='password']:focus,
select:focus {
  outline: 2px solid var(--p-primary-color, #6366f1);
  outline-offset: 1px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.5rem;
}
</style>

