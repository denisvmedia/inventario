<template>
  <div class="restore-list">
    <div class="header">
      <div class="header-title">
        <h1>Restore Operations</h1>
        <div v-if="restores.length > 0" class="item-count">
          {{ restores.length }} restore operation{{ restores.length !== 1 ? 's' : '' }}
        </div>
      </div>
      <div class="header-actions">
        <router-link to="/restores/new" class="btn btn-primary">
          <i class="pi pi-plus"></i> New
        </router-link>
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
          <router-link to="/restores/new" class="btn btn-primary">
            <i class="pi pi-plus"></i> Create Restore
          </router-link>
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
import type { Import } from '@/types';

const router = useRouter();
const toast = useToast();

const restores = ref<Import[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
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



onMounted(() => {
  loadRestores();
});
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.restore-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

// Header styles are now in shared _header.scss

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
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.error-state {
  color: $danger-color;
}

.empty-content {
  max-width: 400px;
}

.empty-content h3 {
  margin: 1rem 0 0.5rem 0;
  color: $text-color;
}

.empty-content p {
  margin-bottom: 1.5rem;
  color: $text-secondary-color;
}

.restore-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 1.5rem;
}

.restore-card {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
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
  border-bottom: 1px solid $border-color;
}

.status-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.restore-type {
  font-size: 0.875rem;
  color: $text-secondary-color;
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
  color: $text-secondary-color;
}

.detail-row .value {
  color: $text-color;
}

.stats-summary {
  display: flex;
  gap: 1rem;
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid $border-color;
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
  color: $primary-color;
}

.stat-label {
  font-size: 0.75rem;
  color: $text-secondary-color;
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
  color: $text-secondary-color;
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
  color: #fd7e14; // Orange color for warnings
  font-weight: 500;
}

.btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: $default-radius;
  font-weight: 500;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  transition: all 0.2s;

  &:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
}

.btn-primary {
  background-color: $primary-color;
  color: white;

  &:hover:not(:disabled) {
    background-color: darken($primary-color, 10%);
  }
}
</style>
