<template>
  <div class="resource-not-found">
    <div class="error-icon">
      <AlertTriangle class="size-16" aria-hidden="true" />
    </div>
    <h3>{{ title }}</h3>
    <p>{{ message }}</p>
    <div class="error-actions">
      <Button
        v-if="showGoBack"
        variant="outline"
        @click="$emit('go-back')"
      >
        <ArrowLeft class="size-4" aria-hidden="true" />
        {{ goBackText }}
      </Button>
      <Button
        v-if="showTryAgain"
        @click="$emit('try-again')"
      >
        <RotateCw class="size-4" aria-hidden="true" />
        {{ tryAgainText }}
      </Button>
      <slot name="custom-actions"></slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, ArrowLeft, RotateCw } from 'lucide-vue-next'

import { Button } from '@design/ui/button'

interface Props {
  resourceType?: string
  title?: string
  message?: string
  showGoBack?: boolean
  showTryAgain?: boolean
  goBackText?: string
  tryAgainText?: string
}

const props = withDefaults(defineProps<Props>(), {
  resourceType: 'resource',
  title: '',
  message: '',
  showGoBack: true,
  showTryAgain: true,
  goBackText: 'Go Back',
  tryAgainText: 'Try Again'
})

defineEmits<{
  'go-back': []
  'try-again': []
}>()

const computedTitle = computed(() => {
  if (props.title) return props.title
  return `Error Loading ${props.resourceType.charAt(0).toUpperCase() + props.resourceType.slice(1)}`
})

const computedMessage = computed(() => {
  if (props.message) return props.message
  return `The ${props.resourceType} was not found. It may have been deleted or moved.`
})

const title = computedTitle
const message = computedMessage
</script>

<style scoped>
.resource-not-found {
  text-align: center;
  padding: 3rem 2rem;
  background: hsl(var(--card));
  border-radius: 0.375rem;
  box-shadow: 0 2px 8px rgb(0 0 0 / 10%);
  max-width: 600px;
  margin: 2rem auto;
}

.error-icon {
  color: hsl(var(--destructive));
  margin-bottom: 1.5rem;
  display: flex;
  justify-content: center;
}

h3 {
  color: hsl(var(--destructive));
  margin: 0 0 1rem;
  font-size: 1.5rem;
  font-weight: 600;
}

p {
  color: hsl(var(--muted-foreground));
  margin: 0 0 2rem;
  font-size: 1rem;
  line-height: 1.5;
}

.error-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}

@media (width <= 480px) {
  .error-actions {
    flex-direction: column;
    align-items: center;
  }

  .error-actions :deep(button) {
    min-width: 200px;
  }
}
</style>
