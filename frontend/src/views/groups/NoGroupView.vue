<template>
  <div class="no-group" data-testid="no-group-view">
    <div class="no-group__card">
      <div class="no-group__hero" aria-hidden="true">🎉</div>
      <h1>Welcome to Inventario</h1>
      <p class="no-group__lead">
        A group is your inventory space — an organization, household, or project.
        Every location, commodity, and file lives inside one.
      </p>
      <p class="no-group__message">
        To get started, create your first group. Already invited by someone?
        Open the invite link they sent you instead.
      </p>
      <div class="no-group__actions">
        <button
          v-if="!showCreateForm"
          class="btn btn-primary"
          data-testid="no-group-create-button"
          @click="showCreateForm = true"
        >
          Create your first group
        </button>
      </div>
      <div v-if="showCreateForm" class="no-group__form">
        <div class="form-group">
          <label for="group-name">Group Name</label>
          <input
            id="group-name"
            v-model="groupName"
            type="text"
            class="form-input"
            placeholder="e.g. My Inventory"
            maxlength="100"
            data-testid="no-group-name-input"
            @keyup.enter="createGroup"
          />
        </div>
        <div class="form-group">
          <label for="group-icon">Icon (optional)</label>
          <input
            id="group-icon"
            v-model="groupIcon"
            type="text"
            class="form-input"
            placeholder="e.g. 📦"
            maxlength="10"
          />
        </div>
        <div class="form-group">
          <label for="group-currency">Main Currency</label>
          <CurrencySelect
            id="group-currency"
            v-model="groupCurrency"
          />
          <small class="no-group__hint">
            Defaults to USD. Immutable after creation — see
            <a href="https://github.com/denisvmedia/inventario/issues/202" target="_blank" rel="noopener">#202</a>.
          </small>
        </div>
        <div class="no-group__form-actions">
          <button class="btn btn-secondary" @click="showCreateForm = false">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="!groupName.trim() || isCreating"
            data-testid="no-group-submit"
            @click="createGroup"
          >
            {{ isCreating ? 'Creating...' : 'Create Group' }}
          </button>
        </div>
        <p v-if="error" class="no-group__error">{{ error }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useGroupStore } from '@/stores/groupStore'
import CurrencySelect from '@/components/CurrencySelect.vue'

const groupStore = useGroupStore()
const router = useRouter()

const showCreateForm = ref(false)
const groupName = ref('')
const groupIcon = ref('')
const groupCurrency = ref('')
const isCreating = ref(false)
const error = ref<string | null>(null)

async function createGroup() {
  if (!groupName.value.trim()) return

  isCreating.value = true
  error.value = null

  try {
    const group = await groupStore.createGroup(
      groupName.value.trim(),
      groupIcon.value.trim() || undefined,
      groupCurrency.value.trim().toUpperCase() || undefined,
    )
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
.no-group {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;

  &__card {
    text-align: center;
    max-width: 520px;
    padding: 2.5em 2em;
    background: white;
    border-radius: 12px;
    box-shadow: 0 2px 8px rgb(0 0 0 / 10%);
  }

  &__hero {
    font-size: 3em;
    line-height: 1;
    margin-bottom: 0.3em;
  }

  &__lead {
    color: #444;
    margin: 1em 0 0.5em;
    line-height: 1.5;
  }

  &__message {
    color: #666;
    margin: 0 0 1.5em;
    line-height: 1.5;
  }

  &__form {
    text-align: left;
    margin-top: 1.5em;

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
        font-size: 0.95em;
      }
    }
  }

  &__form-actions {
    display: flex;
    gap: 0.5em;
    justify-content: flex-end;
    margin-top: 1em;
  }

  &__error {
    color: #c00;
    font-size: 0.9em;
    margin-top: 0.5em;
  }

  &__hint {
    display: block;
    color: #888;
    font-size: 0.85em;
    margin-top: 0.3em;

    a {
      color: #1a73e8;
      text-decoration: none;
      &:hover { text-decoration: underline; }
    }
  }
}

// .btn / .btn-primary / .btn-secondary come from shared _components.scss.
</style>
