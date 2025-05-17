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
        <p>{{ message }}</p>
      </div>
    </div>
    <template #footer>
      <div class="confirmation-actions">
        <button class="btn btn-secondary" @click="cancel">{{ cancelLabel }}</button>
        <button :class="['btn', `btn-${confirmButtonClass}`]" @click="confirm">{{ confirmLabel }}</button>
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
    default: false,
    required: true
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

  p {
    margin: 0;
    line-height: 1.5;
  }
}

.confirmation-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
