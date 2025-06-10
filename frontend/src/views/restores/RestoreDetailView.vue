<template>
  <div class="restore-detail">
    <div class="header">
      <div class="header-title">
        <Button
          icon="pi pi-arrow-left"
          label="Back to Restores"
          class="p-button-text"
          @click="$router.push('/restores')"
        />
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-trash"
          label="Delete"
          class="p-button-danger p-button-outlined"
          @click="confirmDelete"
          :disabled="restore?.status === 'running'"
        />
      </div>
    </div>

    <div class="content-area">
      <div v-if="loading" class="loading-state">
        <ProgressSpinner />
        <p>Loading restore details...</p>
      </div>

      <div v-else-if="error" class="error-state">
        <Message severity="error" :closable="false">
          {{ error }}
        </Message>
      </div>

      <div v-else-if="restore" class="restore-content">
        <!-- Header Section -->
        <Card class="header-card">
          <template #title>
            <div class="restore-header">
              <div class="title-section">
                <h2>{{ restore.description }}</h2>
                <Badge
                  :value="RestoreService.getStatusText(restore.status)"
                  :class="RestoreService.getStatusBadgeClass(restore.status)"
                  class="status-badge"
                />
              </div>
              <div class="type-section">
                <span class="restore-type">{{ restore.type }}</span>
              </div>
            </div>
          </template>

          <template #content>
            <div class="restore-info">
              <div class="info-grid">
                <div class="info-item">
                  <span class="label">Created:</span>
                  <span class="value">{{ RestoreService.formatDate(restore.created_date) }}</span>
                </div>
                
                <div v-if="restore.started_date" class="info-item">
                  <span class="label">Started:</span>
                  <span class="value">{{ RestoreService.formatDate(restore.started_date) }}</span>
                </div>
                
                <div v-if="restore.completed_date" class="info-item">
                  <span class="label">Completed:</span>
                  <span class="value">{{ RestoreService.formatDate(restore.completed_date) }}</span>
                </div>
                
                <div v-if="restore.started_date && restore.completed_date" class="info-item">
                  <span class="label">Duration:</span>
                  <span class="value">{{ RestoreService.calculateDuration(restore.started_date, restore.completed_date) }}</span>
                </div>

                <div class="info-item">
                  <span class="label">Source File:</span>
                  <span class="value">{{ restore.source_file_path }}</span>
                </div>
              </div>

              <div v-if="restore.status === 'running'" class="progress-section">
                <h4>Progress</h4>
                <ProgressBar mode="indeterminate" />
                <p class="progress-text">Restore operation in progress...</p>
              </div>

              <div v-if="restore.error_message" class="error-section">
                <h4>Error Details</h4>
                <Message severity="error" :closable="false">
                  {{ restore.error_message }}
                </Message>
              </div>
            </div>
          </template>
        </Card>

        <!-- Statistics Section -->
        <Card v-if="restore.status === 'completed'" class="stats-card">
          <template #title>
            <h3>Restore Statistics</h3>
          </template>

          <template #content>
            <div class="stats-grid">
              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-map-marker"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.location_count }}</div>
                  <div class="stat-label">Locations</div>
                </div>
              </div>

              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-th-large"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.area_count }}</div>
                  <div class="stat-label">Areas</div>
                </div>
              </div>

              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-box"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.commodity_count }}</div>
                  <div class="stat-label">Commodities</div>
                </div>
              </div>

              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-image"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.image_count }}</div>
                  <div class="stat-label">Images</div>
                </div>
              </div>

              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-file"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.invoice_count }}</div>
                  <div class="stat-label">Invoices</div>
                </div>
              </div>

              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-book"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.manual_count }}</div>
                  <div class="stat-label">Manuals</div>
                </div>
              </div>

              <div class="stat-card">
                <div class="stat-icon">
                  <i class="pi pi-database"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ RestoreService.formatFileSize(restore.binary_data_size) }}</div>
                  <div class="stat-label">Binary Data</div>
                </div>
              </div>

              <div v-if="restore.error_count > 0" class="stat-card error-stat">
                <div class="stat-icon">
                  <i class="pi pi-exclamation-triangle"></i>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ restore.error_count }}</div>
                  <div class="stat-label">Errors</div>
                </div>
              </div>
            </div>
          </template>
        </Card>

        <!-- Errors Section -->
        <Card v-if="restore.errors && restore.errors.length > 0" class="errors-card">
          <template #title>
            <h3>Errors ({{ restore.errors.length }})</h3>
          </template>

          <template #content>
            <div class="errors-list">
              <Message
                v-for="(error, index) in restore.errors"
                :key="index"
                severity="error"
                :closable="false"
                class="error-item"
              >
                {{ error }}
              </Message>
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
          <p><strong>{{ restore?.description }}</strong></p>
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
import { useRoute, useRouter } from 'vue-router';
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

const route = useRoute();
const router = useRouter();
const toast = useToast();

const restore = ref<Import | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const showDeleteDialog = ref(false);
const deleting = ref(false);

const loadRestore = async () => {
  try {
    loading.value = true;
    error.value = null;
    const id = route.params.id as string;
    restore.value = await RestoreService.get(id);
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load restore details';
    console.error('Failed to load restore:', err);
  } finally {
    loading.value = false;
  }
};

const confirmDelete = () => {
  showDeleteDialog.value = true;
};

const deleteRestore = async () => {
  if (!restore.value) return;

  try {
    deleting.value = true;
    await RestoreService.delete(restore.value.id);
    
    showDeleteDialog.value = false;
    
    toast.add({
      severity: 'success',
      summary: 'Success',
      detail: 'Restore operation deleted successfully',
      life: 3000,
    });
    
    router.push('/restores');
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
  loadRestore();
});
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.restore-detail {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

// Header styles are now in shared _header.scss

.content-area {
  min-height: 400px;
}

.loading-state,
.error-state {
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

.restore-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.header-card {
  margin-bottom: 0;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.restore-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.title-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.title-section h2 {
  margin: 0;
  color: $text-color;
}

.status-badge {
  align-self: flex-start;
}

.restore-type {
  font-size: 0.875rem;
  color: $text-secondary-color;
  text-transform: uppercase;
  font-weight: 500;
}

.restore-info {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1rem;
}

.info-item {
  display: flex;
  justify-content: space-between;
  padding: 0.75rem;
  background: $light-bg-color;
  border-radius: $default-radius;
}

.info-item .label {
  font-weight: 500;
  color: $text-secondary-color;
}

.info-item .value {
  color: $text-color;
  text-align: right;
}

.progress-section h4,
.error-section h4 {
  margin: 0 0 1rem 0;
  color: $text-color;
}

.progress-text {
  margin-top: 0.5rem;
  text-align: center;
  color: $text-secondary-color;
  font-size: 0.875rem;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1.5rem;
  background: $light-bg-color;
  border-radius: $default-radius;
  border: 1px solid $border-color;
}

.stat-card.error-stat {
  background: #ffeaea; // Light red background
  border-color: #f5c6cb; // Light red border
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: $primary-color;
  color: white;
  border-radius: 50%;
  font-size: 1.25rem;
}

.error-stat .stat-icon {
  background: $danger-color;
}

.stat-content {
  display: flex;
  flex-direction: column;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 600;
  color: $text-color;
  line-height: 1;
}

.stat-label {
  font-size: 0.875rem;
  color: $text-secondary-color;
  text-transform: uppercase;
  margin-top: 0.25rem;
}

.errors-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.error-item {
  margin: 0;
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
</style>
