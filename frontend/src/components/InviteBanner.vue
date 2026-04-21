<template>
  <div class="invite-banner" data-testid="invite-banner">
    <font-awesome-icon icon="user-plus" />
    <span>
      <slot :group-name="displayName">
        {{ prefix }}
        <strong>{{ displayName }}</strong>.
      </slot>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  groupName?: string | null
  // The leading phrase (e.g. "Sign in to accept the invitation to"). Views
  // that want more control can use the default slot instead, which receives
  // the resolved displayName as a slot prop.
  prefix?: string
  fallback?: string
}

const props = withDefaults(defineProps<Props>(), {
  groupName: null,
  prefix: '',
  fallback: 'the invited group',
})

const displayName = computed(() => props.groupName || props.fallback)
</script>

<style scoped>
.invite-banner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background-color: #e8f4fd;
  color: #144b7a;
  padding: 0.75rem 1rem;
  border-radius: 4px;
  border: 1px solid #b6daf5;
  font-size: 0.9rem;
  margin-bottom: 1rem;
}
</style>
