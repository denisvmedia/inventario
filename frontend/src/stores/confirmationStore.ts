import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useConfirmationStore = defineStore('confirmation', () => {
  // State
  const isVisible = ref(false);
  const title = ref('Confirm Action');
  const message = ref('Are you sure you want to proceed?');
  const confirmLabel = ref('Confirm');
  const cancelLabel = ref('Cancel');
  const confirmButtonClass = ref('primary');

  // Callback functions
  // eslint-disable-next-line no-unused-vars
  let resolvePromise: ((value: boolean) => void) | null = null;

  // Actions
  function show(options: {
    title?: string;
    message?: string;
    confirmLabel?: string;
    cancelLabel?: string;
    confirmButtonClass?: string;
  } = {}) {
    // Set dialog options
    title.value = options.title || 'Confirm Action';
    message.value = options.message || 'Are you sure you want to proceed?';
    confirmLabel.value = options.confirmLabel || 'Confirm';
    cancelLabel.value = options.cancelLabel || 'Cancel';
    confirmButtonClass.value = options.confirmButtonClass || 'primary';

    // Show the dialog
    isVisible.value = true;

    // Return a promise that will be resolved when the user confirms or cancels
    return new Promise<boolean>((resolve) => {
      resolvePromise = resolve;
    });
  }

  function hide() {
    isVisible.value = false;
  }

  function confirm() {
    if (resolvePromise) {
      resolvePromise(true);
      resolvePromise = null;
    }
    hide();
  }

  function cancel() {
    if (resolvePromise) {
      resolvePromise(false);
      resolvePromise = null;
    }
    hide();
  }

  return {
    // State
    isVisible,
    title,
    message,
    confirmLabel,
    cancelLabel,
    confirmButtonClass,

    // Actions
    show,
    hide,
    confirm,
    cancel
  };
});
