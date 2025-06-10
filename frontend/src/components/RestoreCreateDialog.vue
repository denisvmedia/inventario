<template>
  <Dialog
    v-model:visible="dialogVisible"
    header="Create Restore Operation"
    :modal="true"
    :closable="true"
    :style="{ width: '600px' }"
    class="restore-create-dialog"
  >
    <form @submit.prevent="createRestore" class="restore-form">
      <!-- Step 1: File Upload -->
      <div class="form-section">
        <h4>1. Upload XML Backup File</h4>
        <div class="upload-area">
          <FileUpload
            ref="fileUpload"
            mode="basic"
            accept=".xml"
            :maxFileSize="100000000"
            :auto="false"
            chooseLabel="Choose XML File"
            @select="onFileSelect"
            @clear="onFileClear"
            class="file-upload"
          />
          <div v-if="selectedFile" class="file-info">
            <i class="pi pi-file"></i>
            <span>{{ selectedFile.name }}</span>
            <span class="file-size">({{ formatFileSize(selectedFile.size) }})</span>
          </div>
        </div>
      </div>

      <!-- Step 2: Description -->
      <div class="form-section">
        <h4>2. Description</h4>
        <InputText
          v-model="form.description"
          placeholder="Enter a description for this restore operation"
          class="w-full"
          :class="{ 'p-invalid': errors.description }"
        />
        <small v-if="errors.description" class="p-error">{{ errors.description }}</small>
      </div>

      <!-- Step 3: Restore Strategy -->
      <div class="form-section">
        <h4>3. Restore Strategy</h4>
        <div class="strategy-options">
          <div class="strategy-option">
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
          
          <div class="strategy-option">
            <RadioButton
              v-model="form.options.strategy"
              inputId="strategy-merge-add"
              value="merge_add"
            />
            <label for="strategy-merge-add" class="strategy-label">
              <strong>Merge Add</strong>
              <span class="strategy-description">
                Only add data from backup that doesn't exist in current database
              </span>
            </label>
          </div>
          
          <div class="strategy-option">
            <RadioButton
              v-model="form.options.strategy"
              inputId="strategy-merge-update"
              value="merge_update"
            />
            <label for="strategy-merge-update" class="strategy-label">
              <strong>Merge Update</strong>
              <span class="strategy-description">
                Add missing data and update existing data from backup
              </span>
            </label>
          </div>
        </div>
      </div>

      <!-- Step 4: Options -->
      <div class="form-section">
        <h4>4. Options</h4>
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
                Preview what would be restored without making changes
              </span>
            </label>
          </div>
        </div>
      </div>

      <!-- Warning Messages -->
      <div v-if="form.options.strategy === 'full_replace' && !form.options.dry_run" class="warning-section">
        <Message severity="warn" :closable="false">
          <strong>Warning:</strong> Full replace will permanently delete all existing data.
          {{ form.options.backup_existing ? 'A backup will be created first.' : 'Consider enabling backup option.' }}
        </Message>
      </div>

      <div v-if="form.options.dry_run" class="info-section">
        <Message severity="info" :closable="false">
          <strong>Dry Run Mode:</strong> This will preview the restore operation without making any changes to your data.
        </Message>
      </div>
    </form>

    <template #footer>
      <Button
        label="Cancel"
        icon="pi pi-times"
        class="p-button-text"
        @click="closeDialog"
      />
      <Button
        :label="form.options.dry_run ? 'Preview Restore' : 'Start Restore'"
        :icon="form.options.dry_run ? 'pi pi-eye' : 'pi pi-upload'"
        class="p-button-primary"
        @click="createRestore"
        :loading="creating"
        :disabled="!canCreate"
      />
    </template>
  </Dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useToast } from 'primevue/usetoast';
import Dialog from 'primevue/dialog';
import Button from 'primevue/button';
import InputText from 'primevue/inputtext';
import RadioButton from 'primevue/radiobutton';
import Checkbox from 'primevue/checkbox';
import FileUpload from 'primevue/fileupload';
import Message from 'primevue/message';
import { RestoreService } from '@/services/restoreService';
import type { Import, RestoreRequest } from '@/types';

interface Props {
  visible: boolean;
}

interface Emits {
  (e: 'update:visible', value: boolean): void;
  (e: 'created', restore: Import): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();
const toast = useToast();

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const selectedFile = ref<File | null>(null);
const uploadedFilename = ref<string>('');
const creating = ref(false);

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

const errors = ref<Record<string, string>>({});

const canCreate = computed(() => {
  return selectedFile.value && 
         form.value.description.trim() && 
         uploadedFilename.value &&
         !creating.value;
});

const onFileSelect = async (event: any) => {
  const file = event.files[0];
  if (!file) return;

  selectedFile.value = file;
  
  try {
    const response = await RestoreService.uploadFile(file);
    uploadedFilename.value = response.filename;
    form.value.source_file_path = response.filename;
    
    toast.add({
      severity: 'success',
      summary: 'Success',
      detail: 'File uploaded successfully',
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
  }
};

const onFileClear = () => {
  selectedFile.value = null;
  uploadedFilename.value = '';
  form.value.source_file_path = '';
};

const formatFileSize = (bytes: number): string => {
  return RestoreService.formatFileSize(bytes);
};

const validateForm = (): boolean => {
  errors.value = {};
  
  if (!form.value.description.trim()) {
    errors.value.description = 'Description is required';
  }
  
  if (!uploadedFilename.value) {
    errors.value.file = 'XML file is required';
  }
  
  return Object.keys(errors.value).length === 0;
};

const createRestore = async () => {
  if (!validateForm()) return;
  
  try {
    creating.value = true;
    const restore = await RestoreService.create(form.value);
    emit('created', restore);
    resetForm();
  } catch (err) {
    toast.add({
      severity: 'error',
      summary: 'Error',
      detail: err instanceof Error ? err.message : 'Failed to create restore operation',
      life: 5000,
    });
  } finally {
    creating.value = false;
  }
};

const closeDialog = () => {
  dialogVisible.value = false;
  resetForm();
};

const resetForm = () => {
  form.value = {
    description: '',
    source_file_path: '',
    options: {
      strategy: 'merge_add',
      include_file_data: true,
      dry_run: false,
      backup_existing: false,
    },
  };
  selectedFile.value = null;
  uploadedFilename.value = '';
  errors.value = {};
};

// Watch strategy changes to auto-disable backup option
watch(() => form.value.options.strategy, (newStrategy) => {
  if (newStrategy !== 'full_replace') {
    form.value.options.backup_existing = false;
  }
});
</script>

<style scoped>
.restore-form {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.form-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.form-section h4 {
  margin: 0;
  color: var(--text-color);
  font-size: 1rem;
  font-weight: 600;
}

.upload-area {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  background: var(--surface-100);
  border-radius: var(--border-radius);
  color: var(--text-color);
}

.file-size {
  color: var(--text-color-secondary);
  font-size: 0.875rem;
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
  border: 1px solid var(--surface-border);
  border-radius: var(--border-radius);
  cursor: pointer;
  transition: border-color 0.2s;
}

.strategy-option:hover {
  border-color: var(--primary-color);
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
  color: var(--text-color-secondary);
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
  color: var(--text-color-secondary);
}

.warning-section,
.info-section {
  margin-top: 1rem;
}
</style>
