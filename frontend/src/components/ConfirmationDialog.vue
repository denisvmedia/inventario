<template>
  <Dialog
    v-model:visible="visible"
    :header="title"
    :style="{ width: '450px' }"
    :modal="true"
    :closable="true"
    :closeOnEscape="true"
    class="confirmation-modal"
  >
    <div class="confirmation-content">
      <p>{{ message }}</p>
    </div>
    <template #footer>
      <div class="confirmation-actions">
        <button class="btn btn-secondary" @click="onCancel">{{ cancelLabel }}</button>
        <button :class="['btn', `btn-${confirmButtonClass}`]" @click="onConfirm">{{ confirmLabel }}</button>
      </div>
    </template>
  </Dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useConfirmationStore } from '@/stores/confirmationStore';

const confirmationStore = useConfirmationStore();

// Computed properties to access store state
const visible = computed({
  get: () => confirmationStore.isVisible,
  set: (value) => {
    if (!value) confirmationStore.hide();
  }
});

const title = computed(() => confirmationStore.title);
const message = computed(() => confirmationStore.message);
const confirmLabel = computed(() => confirmationStore.confirmLabel);
const cancelLabel = computed(() => confirmationStore.cancelLabel);
const confirmButtonClass = computed(() => confirmationStore.confirmButtonClass);

// Methods
const onConfirm = () => {
  confirmationStore.confirm();
};

const onCancel = () => {
  confirmationStore.cancel();
};
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
