<template>
  <div class="group-create container">
    <h1>Create a New Group</h1>
    <form class="group-form" @submit.prevent="handleCreate">
      <div class="form-group">
        <label for="name">Group Name</label>
        <input id="name" v-model="name" type="text" class="form-input" placeholder="e.g. Home Inventory" maxlength="100" required />
      </div>
      <div class="form-group">
        <label for="icon">Icon (optional)</label>
        <input id="icon" v-model="icon" type="text" class="form-input" placeholder="e.g. 🏠" maxlength="10" />
        <small>Emoji or glyph identifier</small>
      </div>
      <div class="form-group">
        <label for="main-currency">Main Currency</label>
        <input
          id="main-currency"
          v-model="mainCurrency"
          type="text"
          class="form-input"
          placeholder="USD"
          maxlength="3"
        />
        <small>ISO 4217 code (e.g. USD, EUR, CZK). Defaults to USD. Immutable after creation — see <a href="https://github.com/denisvmedia/inventario/issues/202" target="_blank" rel="noopener">#202</a>.</small>
      </div>
      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="router.back()">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="!name.trim() || isCreating">
          {{ isCreating ? 'Creating...' : 'Create Group' }}
        </button>
      </div>
      <p v-if="error" class="error-message">{{ error }}</p>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useGroupStore } from '@/stores/groupStore'

const router = useRouter()
const groupStore = useGroupStore()

const name = ref('')
const icon = ref('')
const mainCurrency = ref('')
const isCreating = ref(false)
const error = ref<string | null>(null)

async function handleCreate() {
  if (!name.value.trim()) return
  isCreating.value = true
  error.value = null
  try {
    const group = await groupStore.createGroup(
      name.value.trim(),
      icon.value.trim() || undefined,
      mainCurrency.value.trim().toUpperCase() || undefined,
    )
    await groupStore.fetchGroups()
    await groupStore.setCurrentGroup(group.slug)
    router.push('/')
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to create group'
  } finally {
    isCreating.value = false
  }
}
</script>

<style scoped lang="scss">
.group-form {
  max-width: 500px;

  .form-group {
    margin-bottom: 1em;

    label {
      display: block;
      margin-bottom: 0.3em;
      font-weight: 500;
    }

    .form-input {
      width: 100%;
      padding: 0.5em;
      border: 1px solid #ccc;
      border-radius: 6px;
    }

    small {
      color: #888;
      font-size: 0.85em;
    }
  }

  .form-actions {
    display: flex;
    gap: 0.5em;
    margin-top: 1.5em;
  }

  .error-message {
    color: #c00;
    margin-top: 0.5em;
  }
}

// .btn / .btn-primary / .btn-secondary come from shared _components.scss.
</style>
