<template>
  <PageContainer width="narrow" class="group-create">
    <PageHeader title="Create a New Group" />
    <form class="group-form flex max-w-lg flex-col gap-4" @submit.prevent="handleCreate">
      <div class="flex flex-col gap-2">
        <Label for="name">Group Name</Label>
        <Input
          id="name"
          v-model="name"
          type="text"
          placeholder="e.g. Home Inventory"
          maxlength="100"
          required
        />
      </div>
      <div class="flex flex-col gap-2">
        <Label for="group-create-icon-trigger">Icon (optional)</Label>
        <IconPicker
          v-model="icon"
          trigger-id="group-create-icon-trigger"
          trigger-label="Choose an icon"
          panel-aria-label="Pick a group icon"
          trigger-test-id="group-create-icon-picker"
        />
        <small class="text-xs text-muted-foreground">Pick an emoji that represents this group.</small>
      </div>
      <div class="flex flex-col gap-2">
        <Label for="main-currency">Main Currency</Label>
        <CurrencySelect
          id="main-currency"
          v-model="mainCurrency"
        />
        <small class="text-xs text-muted-foreground">Defaults to USD. Immutable after creation — see <a class="text-primary hover:underline" href="https://github.com/denisvmedia/inventario/issues/202" target="_blank" rel="noopener">#202</a>.</small>
      </div>
      <FormFooter>
        <Button type="button" variant="outline" @click="router.back()">Cancel</Button>
        <Button type="submit" :disabled="!name.trim() || isCreating">
          {{ isCreating ? 'Creating...' : 'Create Group' }}
        </Button>
      </FormFooter>
      <p v-if="error" class="error-message text-sm text-destructive">{{ error }}</p>
    </form>
  </PageContainer>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import FormFooter from '@design/patterns/FormFooter.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useGroupStore } from '@/stores/groupStore'
import CurrencySelect from '@/components/CurrencySelect.vue'
import IconPicker from '@/components/IconPicker.vue'

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
