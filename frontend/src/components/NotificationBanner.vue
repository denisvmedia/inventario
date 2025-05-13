<template>
  <div v-if="show" class="notification-banner" :class="type">
    <div class="notification-content">
      <div class="notification-icon">
        <font-awesome-icon :icon="icon" />
      </div>
      <div class="notification-message">
        <slot></slot>
      </div>
    </div>
    <button v-if="dismissible" class="notification-close" @click="dismiss">
      <font-awesome-icon icon="times" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps({
  type: {
    type: String,
    default: 'info',
    validator: (value: string) => ['info', 'warning', 'error', 'success'].includes(value)
  },
  dismissible: {
    type: Boolean,
    default: true
  },
  autoClose: {
    type: Number,
    default: 0 // 0 means no auto-close
  }
})

const show = ref(true)

const icon = computed(() => {
  switch (props.type) {
    case 'warning':
      return 'exclamation-triangle'
    case 'error':
      return 'exclamation-circle'
    case 'success':
      return 'check-circle'
    case 'info':
    default:
      return 'info-circle'
  }
})

const dismiss = () => {
  show.value = false
}

// Auto-close functionality
if (props.autoClose > 0) {
  setTimeout(() => {
    show.value = false
  }, props.autoClose)
}
</script>

<style lang="scss" scoped>
@import '@/assets/main.scss';

.notification-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  
  &.info {
    background-color: #cce5ff;
    border: 1px solid #b8daff;
    color: #004085;
  }
  
  &.warning {
    background-color: #fff3cd;
    border: 1px solid #ffeeba;
    color: #856404;
  }
  
  &.error {
    background-color: #f8d7da;
    border: 1px solid #f5c6cb;
    color: #721c24;
  }
  
  &.success {
    background-color: #d4edda;
    border: 1px solid #c3e6cb;
    color: #155724;
  }
}

.notification-content {
  display: flex;
  align-items: center;
  flex: 1;
}

.notification-icon {
  margin-right: 0.75rem;
}

.notification-message {
  flex: 1;
}

.notification-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.25rem;
  margin-left: 0.5rem;
  opacity: 0.7;
  
  &:hover {
    opacity: 1;
  }
}
</style>
