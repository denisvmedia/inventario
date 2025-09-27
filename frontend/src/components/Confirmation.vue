<template>
  <Dialog
    v-model:visible="dialogVisible"
    :header="title"
    :style="{ width: '450px' }"
    :modal="true"
    :closable="true"
    :closeOnEscape="true"
    :dismissable-mask="true"
    :class="['confirmation-modal', `p-confirm-dialog-${confirmButtonClass}`]"
  >
    <div class="confirmation-content">
      <font-awesome-icon v-if="confirmationIcon" :icon="confirmationIcon" class="confirmation-icon" />
      <div class="confirmation-message">
        <!-- eslint-disable-next-line vue/no-v-html -->
        <p v-html="message"></p>
      </div>
    </div>
    <template #footer>
      <div class="confirmation-actions">
        <button :class="['btn', `btn-${confirmButtonClass}`]" :disabled="confirmDisabled" @click="confirm">{{ confirmLabel }}</button>
        <button class="btn btn-secondary" @click="cancel">{{ cancelLabel }}</button>
      </div>
    </template>
  </Dialog>
</template>

<script setup lang="ts">
import {isIconRegistered} from "@/utils/faHelper.ts";
import {computed} from "vue";

const props = defineProps({
  cancelLabel: {
    type: String,
    required: true
  },
  confirmLabel: {
    type: String,
    required: true
  },
  confirmButtonClass: {
    type: String,
    default: 'primary',
    required: false,
    validator: (value: string) => {
      if (!value) return true
      return ['primary', 'danger', 'warning', 'success', 'secondary'].includes(value)
    }
  },
  confirmDisabled: {
    type: Boolean,
    default: false,
    required: false
  },
  confirmationIcon: {
    type: String,
    required: false,
    validator: (value: string) => {
      if (!value) return true
      return isIconRegistered(value)
    }
  },
  message: {
    type: String,
    required: true
  },
  title: {
    type: String,
    required: true
  },
  visible: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:visible', 'cancel', 'confirm'])

const cancel = () => {
  emit('cancel')
  emit('update:visible', false)
}

const confirm = () => {
  emit('confirm')
}

const dialogVisible = computed({
  get: () => props.visible,
  set: (val: boolean) => {
    // if (!val) {
    //   emit('cancel') // реагируем на закрытие
    // }
    emit('update:visible', val)
  }
})
</script>

<style lang="scss" scoped>
@use 'sass:color';
@use '@/assets/variables' as *;

.confirmation-content {
  margin: 1rem 0;
  display: flex;
  align-items: flex-start;
  gap: 1rem;

  .confirmation-icon {
    color: $error-color;
    font-size: 1.5rem;
    margin-top: 0.25rem;
    flex-shrink: 0;
  }

  .confirmation-message {
    flex: 1;

    p {
      margin: 0 0 1rem;
      line-height: 1.5;
      color: $text-color;

      &:last-child {
        margin-bottom: 0;
      }

      // Style for warning text
      :deep(.warning-text) {
        color: $error-color;
        font-weight: 500;
      }
    }
  }
}

.confirmation-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
}

// Override PrimeVue Dialog styles to match original modal appearance
:deep(.p-dialog) {
  .p-dialog-header {
    padding: 1.5rem;
    border-bottom: 1px solid $border-color;

    .p-dialog-title {
      color: $text-color;
      font-size: 1.25rem;
      font-weight: 600;
    }
  }

  .p-dialog-content {
    padding: 1.5rem;
  }

  .p-dialog-footer {
    padding: 1.5rem;
    border-top: 1px solid $border-color;
  }
}

// Ensure danger confirmation styling
.confirmation-modal.p-confirm-dialog-danger {
  :deep(.p-dialog-header) {
    .p-dialog-title {
      color: $text-color;
    }
  }
}
</style>
