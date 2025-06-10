<template>
  <div class="restore-list">
    <div class="control-plane">
      <div class="control-plane-left">
        <h2>Restore Operations</h2>
      </div>
      <div class="control-plane-right">
        <Button
          label="+ New Restore"
          icon="pi pi-plus"
          @click="showCreateDialog = true"
          class="p-button-primary"
        />
      </div>
    </div>

    <div class="content-area">
      <div v-if="loading" class="loading-state">
        <ProgressSpinner />
        <p>Loading restore operations...</p>
      </div>

      <div v-else-if="error" class="error-state">
        <Message severity="error" :closable="false">
          {{ error }}
        </Message>
      </div>

      <div v-else-if="restores.length === 0" class="empty-state">
        <div class="empty-content">
          <i class="pi pi-upload" style="font-size: 3rem; color: var(--text-color-secondary);"></i>
          <h3>No Restore Operations</h3>
          <p>Create your first restore operation to import data from XML backups.</p>
          <Button
            label="Create Restore"
            icon="pi pi-plus"
            @click="showCreateDialog = true"
            class="p-button-primary"
          />
        </div>
      </div>

      <div v-else class="restore-grid">
        <Card
          v-for="restore in restores"
          :key="restore.id"
          class="restore-card"
          @click="$router.push(`/restores/${restore.id}`)"
        >
          <template #header>
            <div class="card-header">
              <div class="status-info">
                <Badge
                  :value="RestoreService.getStatusText(restore.status)"
                  :class="RestoreService.getStatusBadgeClass(restore.status)"
                />
                <span class="restore-type">{{ restore.type }}</span>
              </div>
              <div class="card-actions">
                <Button
                  icon="pi pi-trash"
                  class="p-button-text p-button-danger p-button-sm"
                  @click.stop="confirmDelete(restore)"
                  :disabled="restore.status === 'running'"
                  v-tooltip="'Delete'"
                />
              </div>
            </div>
          </template>

          <template #title>
            {{ restore.description }}
          </template>

          <template #content>
            <div class="restore-details">
              <div class="detail-row">
                <span class="label">Created:</span>
                <span class="value">{{ RestoreService.formatDate(restore.created_date) }}</span>
              </div>
              
              <div v-if="restore.started_date" class="detail-row">
                <span class="label">Started:</span>
                <span class="value">{{ RestoreService.formatDate(restore.started_date) }}</span>
              </div>
              
              <div v-if="restore.completed_date" class="detail-row">
                <span class="label">Completed:</span>
                <span class="value">{{ RestoreService.formatDate(restore.completed_date) }}</span>
              </div>
              
              <div v-if="restore.started_date && restore.completed_date" class="detail-row">
                <span class="label">Duration:</span>
                <span class="value">{{ RestoreService.calculateDuration(restore.started_date, restore.completed_date) }}</span>
              </div>

              <div v-if="restore.status === 'completed'" class="stats-summary">
                <div class="stat-item">
                  <span class="stat-value">{{ restore.location_count }}</span>
                  <span class="stat-label">Locations</span>
                </div>
                <div class="stat-item">
                  <span class="stat-value">{{ restore.area_count }}</span>
                  <span class="stat-label">Areas</span>
                </div>
                <div class="stat-item">
                  <span class="stat-value">{{ restore.commodity_count }}</span>
                  <span class="stat-label">Commodities</span>
                </div>
              </div>

              <div v-if="restore.error_message" class="error-message">
                <Message severity="error" :closable="false">
                  {{ restore.error_message }}
                </Message>
              </div>

              <div v-if="restore.status === 'running'" class="progress-info">
                <ProgressBar mode="indeterminate" />
                <p class="progress-text">Restore in progress...</p>
              </div>
            </div>
          </template>
        </Card>
      </div>
    </div>

    <!-- Create Restore Dialog -->
    <RestoreCreateDialog
      v-model:visible="showCreateDialog"
      @created="onRestoreCreated"
    />

    <!-- Delete Confirmation Dialog -->
    <Dialog
      v-model:visible="showDeleteDialog"
      header="Confirm Delete"
      :modal="true"
      :closable="true"
      class="confirm-dialog"
    >
      <div class="confirmation-content">
        <i class="pi pi-exclamation-triangle" style="font-size: 2rem; color: var(--orange-500);"></i>
        <div class="message">
          <h3>Delete Restore Operation</h3>
          <p>Are you sure you want to delete this restore operation?</p>
          <p><strong>{{ restoreToDelete?.description }}</strong></p>
          <p class="warning-text">This action cannot be undone.</p>
        </div>
      </div>
      <template #footer>
        <Button
          label="Delete"
          icon="pi pi-trash"
          class="p-button-danger"
          @click="deleteRestore"
          :loading="deleting"
        />
        <Button
          label="Cancel"
          icon="pi pi-times"
          class="p-button-text"
          @click="showDeleteDialog = false"
        />
      </template>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { useToast } from 'primevue/usetoast';
import Button from 'primevue/button';
import Card from 'primevue/card';
import Badge from 'primevue/badge';
import Dialog from 'primevue/dialog';
import Message from 'primevue/message';
import ProgressSpinner from 'primevue/progressspinner';
import ProgressBar from 'primevue/progressbar';
import { RestoreService } from '@/services/restoreService';
import RestoreCreateDialog from '@/components/RestoreCreateDialog.vue';
import type { Import } from '@/types';

const router = useRouter();
const toast = useToast();

const restores = ref<Import[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const showCreateDialog = ref(false);
const showDeleteDialog = ref(false);
const restoreToDelete = ref<Import | null>(null);
const deleting = ref(false);

const loadRestores = async () => {
  try {
    loading.value = true;
    error.value = null;
    restores.value = await RestoreService.list();
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load restore operations';
    console.error('Failed to load restores:', err);
  } finally {
    loading.value = false;
  }
};

const confirmDelete = (restore: Import) => {
  restoreToDelete.value = restore;
  showDeleteDialog.value = true;
};

const deleteRestore = async () => {
  if (!restoreToDelete.value) return;

  try {
    deleting.value = true;
    await RestoreService.delete(restoreToDelete.value.id);
    
    restores.value = restores.value.filter(r => r.id !== restoreToDelete.value!.id);
    showDeleteDialog.value = false;
    restoreToDelete.value = null;
    
    toast.add({
      severity: 'success',
      summary: 'Success',
      detail: 'Restore operation deleted successfully',
      life: 3000,
    });
  } catch (err) {
    toast.add({
      severity: 'error',
      summary: 'Error',
      detail: err instanceof Error ? err.message : 'Failed to delete restore operation',
      life: 5000,
    });
  } finally {
    deleting.value = false;
  }
};

const onRestoreCreated = (newRestore: Import) => {
  restores.value.unshift(newRestore);
  showCreateDialog.value = false;
  
  toast.add({
    severity: 'success',
    summary: 'Success',
    detail: 'Restore operation created successfully',
    life: 3000,
  });
};

onMounted(() => {
  loadRestores();
});
</script>

<style scoped>
.restore-list {
  padding: 1rem;
}

.control-plane {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
  padding: 1rem;
  background: var(--surface-card);
  border-radius: var(--border-radius);
  border: 1px solid var(--surface-border);
}

.control-plane-left h2 {
  margin: 0;
  color: var(--text-color);
}

.content-area {
  min-height: 400px;
}

.loading-state,
.error-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  text-align: center;
}

.empty-content {
  max-width: 400px;
}

.empty-content h3 {
  margin: 1rem 0 0.5rem 0;
  color: var(--text-color);
}

.empty-content p {
  margin-bottom: 1.5rem;
  color: var(--text-color-secondary);
}

.restore-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 1.5rem;
}

.restore-card {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.restore-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid var(--surface-border);
}

.status-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.restore-type {
  font-size: 0.875rem;
  color: var(--text-color-secondary);
  text-transform: uppercase;
}

.card-actions {
  display: flex;
  gap: 0.25rem;
}

.restore-details {
  padding: 1rem;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}

.detail-row .label {
  font-weight: 500;
  color: var(--text-color-secondary);
}

.detail-row .value {
  color: var(--text-color);
}

.stats-summary {
  display: flex;
  gap: 1rem;
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--surface-border);
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.stat-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--primary-color);
}

.stat-label {
  font-size: 0.75rem;
  color: var(--text-color-secondary);
  text-transform: uppercase;
}

.error-message {
  margin-top: 1rem;
}

.progress-info {
  margin-top: 1rem;
}

.progress-text {
  margin-top: 0.5rem;
  text-align: center;
  color: var(--text-color-secondary);
  font-size: 0.875rem;
}

.confirmation-content {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
}

.confirmation-content .message h3 {
  margin: 0 0 0.5rem 0;
}

.confirmation-content .message p {
  margin: 0.25rem 0;
}

.warning-text {
  color: var(--orange-500);
  font-weight: 500;
}
</style>
