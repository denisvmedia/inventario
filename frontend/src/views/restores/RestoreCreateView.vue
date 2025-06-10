<template>
  <div class="restore-create">
    <div class="breadcrumb-nav">
      <router-link to="/restores" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Restores
      </router-link>
    </div>
    <h1>Create New Restore</h1>

    <div v-if="error" class="error-message">{{ error }}</div>

    <form @submit.prevent="createRestore" class="restore-form">
      <!-- Step 1: File Upload -->
      <div class="form-section">
        <h2>1. Upload XML Backup File</h2>

        <div class="form-group">
          <label for="file-upload">XML Backup File</label>
          <FileUploader
            :multiple="false"
            accept=".xml,application/xml,text/xml"
            upload-prompt="Drag and drop XML backup file here"
            @upload="onFileUpload"
          />

          <!-- Show selected file info -->
          <div v-if="selectedFile" class="file-info">
            <font-awesome-icon icon="file" />
            <span class="file-name">{{ selectedFile.name }}</span>
            <span class="file-size">({{ formatFileSize(selectedFile.size) }})</span>
            <button type="button" class="remove-file" @click="clearFile">×</button>
          </div>

          <div v-if="formErrors.source_file_path" class="error-message">{{ formErrors.source_file_path }}</div>
          <div class="form-help">Select an XML backup file to restore from</div>
        </div>
      </div>

      <!-- Step 2: Description -->
      <div class="form-section">
        <h2>2. Restore Details</h2>

        <div class="form-group">
          <label for="description">Description</label>
          <textarea
            id="description"
            v-model="form.description"
            placeholder="Enter a description for this restore operation..."
            rows="3"
            maxlength="500"
            required
            :class="{ 'is-invalid': formErrors.description }"
          ></textarea>
          <div v-if="formErrors.description" class="error-message">{{ formErrors.description }}</div>
          <div class="form-help">Describe what this restore operation will do</div>
        </div>
      </div>

      <!-- Step 3: Restore Strategy -->
      <div class="form-section">
        <h2>3. Restore Strategy</h2>

        <div class="strategy-options">
          <div class="strategy-option" :class="{ selected: form.options.strategy === 'merge_add' }">
            <RadioButton
              v-model="form.options.strategy"
              inputId="strategy-merge-add"
              value="merge_add"
            />
            <label for="strategy-merge-add" class="strategy-label">
              <strong>Merge & Add</strong>
              <span class="strategy-description">
                Add new items from backup, keep existing data unchanged
              </span>
            </label>
          </div>

          <div class="strategy-option" :class="{ selected: form.options.strategy === 'merge_update' }">
            <RadioButton
              v-model="form.options.strategy"
              inputId="strategy-merge-update"
              value="merge_update"
            />
            <label for="strategy-merge-update" class="strategy-label">
              <strong>Merge & Update</strong>
              <span class="strategy-description">
                Add new items and update existing ones with backup data
              </span>
            </label>
          </div>

          <div class="strategy-option" :class="{ selected: form.options.strategy === 'full_replace' }">
            <RadioButton
              v-model="form.options.strategy"
              inputId="strategy-full-replace"
              value="full_replace"
            />
            <label for="strategy-full-replace" class="strategy-label">
              <strong>Full Replace</strong>
              <span class="strategy-description">
                Clear all existing data and restore everything from backup
              </span>
            </label>
          </div>
        </div>
        <div v-if="formErrors.strategy" class="error-message">{{ formErrors.strategy }}</div>
      </div>

      <!-- Step 4: Options -->
      <div class="form-section">
        <h2>4. Options</h2>

        <div class="option-group">
          <div class="option-item">
            <Checkbox
              v-model="form.options.include_file_data"
              inputId="include-files"
              binary
            />
            <label for="include-files" class="option-label">
              <strong>Include File Data</strong>
              <span class="option-description">
                Restore images, invoices, and manuals from backup
              </span>
            </label>
          </div>

          <div class="option-item">
            <Checkbox
              v-model="form.options.backup_existing"
              inputId="backup-existing"
              binary
              :disabled="form.options.strategy !== 'full_replace'"
            />
            <label for="backup-existing" class="option-label">
              <strong>Backup Existing Data</strong>
              <span class="option-description">
                Create a backup of current data before full replace
              </span>
            </label>
          </div>

          <div class="option-item">
            <Checkbox
              v-model="form.options.dry_run"
              inputId="dry-run"
              binary
            />
            <label for="dry-run" class="option-label">
              <strong>Dry Run</strong>
              <span class="option-description">
                Preview the restore operation without making changes
              </span>
            </label>
          </div>
        </div>
      </div>

      <div v-if="form.options.dry_run" class="info-section">
        <Message severity="info" :closable="false">
          <strong>Dry Run Mode:</strong> This will preview the restore operation without making any changes to your data.
        </Message>
      </div>

      <div class="form-actions">
        <router-link to="/restores" class="btn btn-secondary">Cancel</router-link>
        <button
          type="submit"
          class="btn btn-primary"
          :disabled="!canSubmit || creating"
        >
          <font-awesome-icon v-if="creating" icon="spinner" spin />
          <font-awesome-icon v-else icon="upload" />
          {{ creating ? 'Creating...' : (form.options.dry_run ? 'Preview Restore' : 'Start Restore') }}
        </button>
      </div>
    </form>

    <div v-if="formError" class="form-error">{{ formError }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { useRouter } from 'vue-router';
import { useToast } from 'primevue/usetoast';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';
import FileUploader from '@/components/FileUploader.vue';
import RadioButton from 'primevue/radiobutton';
import Checkbox from 'primevue/checkbox';
import Message from 'primevue/message';
import { RestoreService } from '@/services/restoreService';
import type { RestoreRequest } from '@/types';

const router = useRouter();
const toast = useToast();

const selectedFile = ref<File | null>(null);
const uploadedFilename = ref<string>('');
const creating = ref(false);
const error = ref<string>('');
const formError = ref<string | null>(null);

const form = ref<RestoreRequest>({
  description: '',
  source_file_path: '',
  options: {
    strategy: 'merge_add',
    include_file_data: true,
    dry_run: false,
    backup_existing: false,
  },
});

const formErrors = ref<Record<string, string>>({});

const canSubmit = computed(() => {
  return selectedFile.value &&
         form.value.description.trim() &&
         uploadedFilename.value &&
         !creating.value;
});

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const onFileUpload = async (files: File[]) => {
  if (files.length === 0) return;

  const file = files[0];
  selectedFile.value = file;

  try {
    const response = await RestoreService.uploadFile(file);
    uploadedFilename.value = response.filename;
    form.value.source_file_path = response.filename;

    // Clear any previous file-related errors
    if (formErrors.value.source_file_path) {
      delete formErrors.value.source_file_path;
    }

    toast.add({
      severity: 'success',
      summary: 'Upload Success',
      detail: 'XML backup file uploaded successfully',
      life: 3000,
    });
  } catch (err) {
    toast.add({
      severity: 'error',
      summary: 'Upload Error',
      detail: err instanceof Error ? err.message : 'Failed to upload file',
      life: 5000,
    });
    selectedFile.value = null;
    uploadedFilename.value = '';
    form.value.source_file_path = '';
  }
};

const clearFile = () => {
  selectedFile.value = null;
  uploadedFilename.value = '';
  form.value.source_file_path = '';

  // Clear any file-related errors
  if (formErrors.value.source_file_path) {
    delete formErrors.value.source_file_path;
  }
};

const validateForm = (): boolean => {
  const errors: Record<string, string> = {};

  if (!form.value.description.trim()) {
    errors.description = 'Description is required';
  }

  if (!uploadedFilename.value) {
    errors.source_file_path = 'XML backup file is required';
  }

  formErrors.value = errors;
  return Object.keys(errors).length === 0;
};

const scrollToFirstError = () => {
  const firstErrorElement = document.querySelector('.is-invalid, .error-message');
  if (firstErrorElement) {
    firstErrorElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }
};

const createRestore = async () => {
  if (!validateForm()) {
    scrollToFirstError();
    return;
  }

  try {
    creating.value = true;
    error.value = '';
    formError.value = null;

    const restore = await RestoreService.create(form.value);

    toast.add({
      severity: 'success',
      summary: 'Success',
      detail: 'Restore operation created successfully',
      life: 3000,
    });

    // Navigate to the restore detail page
    router.push(`/restores/${restore.id}`);
  } catch (err: any) {
    console.error('Error creating restore:', err);

    if (err.response) {
      console.error('Response status:', err.response.status);
      console.error('Response data:', err.response.data);

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {};

      // Map API errors to form fields
      const fieldErrors: Record<string, string> = {};
      const unknownErrors: Record<string, string> = {};

      Object.entries(apiErrors).forEach(([field, message]) => {
        if (['description', 'source_file_path', 'strategy'].includes(field)) {
          fieldErrors[field] = String(message);
        } else {
          unknownErrors[field] = String(message);
        }
      });

      formErrors.value = fieldErrors;

      // If there are field errors, show a general message and scroll to first error
      if (Object.keys(unknownErrors).length === 0) {
        formError.value = 'Please correct the errors above.';
        scrollToFirstError();
      } else {
        formError.value = 'Please correct the errors above. Additional errors: ' + JSON.stringify(unknownErrors);
        scrollToFirstError();
      }
    } else {
      // No field-specific errors, show general error
      formError.value = 'Failed to create restore: ' + (err.message || 'Unknown error');
    }
  } finally {
    creating.value = false;
  }
};
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.restore-create {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

h1 {
  margin: 0 0 30px;
  font-size: 2rem;
}

.error-message {
  background-color: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border-radius: $default-radius;
  margin-bottom: 1rem;
  border: 1px solid #f5c6cb;
}

.restore-form {
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  padding: 30px;
}

.form-section {
  margin-bottom: 30px;
}

.form-section h2 {
  margin: 0 0 20px;
  font-size: 1.5rem;
  color: $text-color;
  border-bottom: 2px solid $primary-color;
  padding-bottom: 10px;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
  color: $text-color;
}

.form-group input,
.form-group select,
.form-group textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  font-size: 1rem;
}

.form-group textarea {
  resize: vertical;
  min-height: 80px;
}

.form-help {
  font-size: 0.85rem;
  color: $text-secondary-color;
  margin-top: 5px;
}

.upload-area {
  border: 2px dashed $border-color;
  border-radius: $default-radius;
  padding: 2rem;
  text-align: center;
  transition: border-color 0.2s, background-color 0.2s;
  background-color: #fafafa;

  &:hover {
    border-color: $primary-color;
    background-color: rgba($primary-color, 0.02);
  }

  .file-upload {
    display: inline-block;
  }
}

.file-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.5rem;
  padding: 0.5rem;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  font-size: 0.875rem;
  border: 1px solid $border-color;
}

.file-name {
  flex: 1;
  font-weight: 500;
}

.file-size {
  color: $text-secondary-color;
}

.remove-file {
  background: none;
  border: none;
  color: $text-secondary-color;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 0.25rem;
  border-radius: 50%;
  width: 1.5rem;
  height: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s, color 0.2s;

  &:hover {
    background-color: rgba($danger-color, 0.1);
    color: $danger-color;
  }
}

.strategy-options {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.strategy-option {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  cursor: pointer;
  transition: border-color 0.2s, background-color 0.2s;

  &:hover {
    border-color: $primary-color;
    background-color: rgba($primary-color, 0.05);
  }

  &.selected {
    border-color: $primary-color;
    background-color: rgba($primary-color, 0.1);
  }
}

.strategy-label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  cursor: pointer;
  flex: 1;
}

.strategy-description {
  font-size: 0.875rem;
  color: $text-secondary-color;
  font-weight: normal;
}

.option-group {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.option-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.option-label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  cursor: pointer;
  flex: 1;
}

.option-description {
  font-size: 0.875rem;
  color: $text-secondary-color;
  font-weight: normal;
}

.info-section {
  margin-top: 1rem;

  :deep(.p-message) {
    border: none;
    box-shadow: none;
  }
}

.form-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid $border-color;
}



.form-error {
  background-color: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border-radius: $default-radius;
  margin-top: 1rem;
  border: 1px solid #f5c6cb;
}
</style>
