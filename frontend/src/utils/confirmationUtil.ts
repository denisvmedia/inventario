import { useConfirmationStore } from '@/stores/confirmationStore';

/**
 * Utility function to show a confirmation dialog
 * 
 * @param options Configuration options for the confirmation dialog
 * @returns Promise that resolves to true if confirmed, false if cancelled
 */
export const confirm = async (options: {
  title?: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  confirmButtonClass?: 'primary' | 'danger' | 'warning' | 'success' | 'secondary';
} = {}): Promise<boolean> => {
  const confirmationStore = useConfirmationStore();
  return confirmationStore.show(options);
};

/**
 * Utility function to show a delete confirmation dialog
 * 
 * @param itemType The type of item being deleted (e.g., 'area', 'location', 'commodity')
 * @returns Promise that resolves to true if confirmed, false if cancelled
 */
export const confirmDelete = async (itemType: string): Promise<boolean> => {
  return confirm({
    title: 'Confirm Delete',
    message: `Are you sure you want to delete this ${itemType}?`,
    confirmLabel: 'Delete',
    cancelLabel: 'Cancel',
    confirmButtonClass: 'danger'
  });
};
