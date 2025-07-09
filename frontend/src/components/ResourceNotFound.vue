<template>
  <div class="resource-not-found">
    <div class="error-icon">
      <font-awesome-icon icon="exclamation-triangle" />
    </div>
    <h3>{{ title }}</h3>
    <p>{{ message }}</p>
    <div class="error-actions">
      <button 
        v-if="showGoBack"
        class="btn btn-secondary" 
        @click="$emit('go-back')"
      >
        <font-awesome-icon icon="arrow-left" />
        {{ goBackText }}
      </button>
      <button 
        v-if="showTryAgain"
        class="btn btn-primary" 
        @click="$emit('try-again')"
      >
        <font-awesome-icon icon="redo" />
        {{ tryAgainText }}
      </button>
      <slot name="custom-actions"></slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

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

// Emit events for parent components to handle
defineEmits<{
  'go-back': []
  'try-again': []
}>()

// Computed properties for default values
const computedTitle = computed(() => {
  if (props.title) return props.title
  return `Error Loading ${props.resourceType.charAt(0).toUpperCase() + props.resourceType.slice(1)}`
})

const computedMessage = computed(() => {
  if (props.message) return props.message
  return `The ${props.resourceType} was not found. It may have been deleted or moved.`
})

// Use computed values in template
const title = computedTitle
const message = computedMessage
</script>

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.resource-not-found {
  text-align: center;
  padding: 3rem 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  max-width: 600px;
  margin: 2rem auto;

  .error-icon {
    font-size: 4rem;
    color: $error-color;
    margin-bottom: 1.5rem;
  }

  h3 {
    color: $error-color;
    margin: 0 0 1rem;
    font-size: 1.5rem;
    font-weight: 600;
  }

  p {
    color: $text-secondary-color;
    margin: 0 0 2rem;
    font-size: 1rem;
    line-height: 1.5;
  }

  .error-actions {
    display: flex;
    gap: 1rem;
    justify-content: center;
    flex-wrap: wrap;

    @media (width <= 480px) {
      flex-direction: column;
      align-items: center;
      
      .btn {
        min-width: 200px;
      }
    }
  }
}
</style>
