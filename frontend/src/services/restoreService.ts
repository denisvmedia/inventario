import type { ApiResponse, ResourceObject, Import, RestoreRequest } from '@/types';

const API_BASE = '/api/v1';

export interface RestoreListResponse {
  data: ResourceObject<Import>[];
}

export interface RestoreResponse {
  data: ResourceObject<Import>;
}

export interface UploadResponse {
  filename: string;
  message: string;
}

export class RestoreService {
  /**
   * Get all restore operations
   */
  static async list(): Promise<Import[]> {
    const response = await fetch(`${API_BASE}/restores`);
    if (!response.ok) {
      throw new Error(`Failed to fetch restores: ${response.statusText}`);
    }
    const data: RestoreListResponse = await response.json();
    return data.data.map(item => item.attributes);
  }

  /**
   * Get a specific restore operation by ID
   */
  static async get(id: string): Promise<Import> {
    const response = await fetch(`${API_BASE}/restores/${id}`);
    if (!response.ok) {
      throw new Error(`Failed to fetch restore: ${response.statusText}`);
    }
    const data: RestoreResponse = await response.json();
    return data.data.attributes;
  }

  /**
   * Create a new restore operation
   */
  static async create(request: RestoreRequest): Promise<Import> {
    const response = await fetch(`${API_BASE}/restores`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        data: {
          type: 'restores',
          attributes: {
            type: 'xml_backup',
            description: request.description,
            source_file_path: request.source_file_path,
          },
        },
        options: request.options,
      }),
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.errors?.[0]?.detail || `Failed to create restore: ${response.statusText}`);
    }

    const data: RestoreResponse = await response.json();
    return data.data.attributes;
  }

  /**
   * Upload an XML file for restore
   */
  static async uploadFile(file: File): Promise<UploadResponse> {
    const formData = new FormData();
    formData.append('files', file);

    const response = await fetch(`${API_BASE}/uploads/restores`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.errors?.[0]?.detail || `Failed to upload file: ${response.statusText}`);
    }

    const result = await response.json();
    // Transform the response to match the expected format
    return {
      filename: result.attributes.fileNames[0],
      message: 'File uploaded successfully'
    };
  }

  /**
   * Delete a restore operation
   */
  static async delete(id: string): Promise<void> {
    const response = await fetch(`${API_BASE}/restores/${id}`, {
      method: 'DELETE',
    });

    if (!response.ok) {
      throw new Error(`Failed to delete restore: ${response.statusText}`);
    }
  }

  /**
   * Get status badge class for restore status
   */
  static getStatusBadgeClass(status: Import['status']): string {
    switch (status) {
      case 'pending':
        return 'p-badge-warning';
      case 'running':
        return 'p-badge-info';
      case 'completed':
        return 'p-badge-success';
      case 'failed':
        return 'p-badge-danger';
      default:
        return 'p-badge-secondary';
    }
  }

  /**
   * Get human-readable status text
   */
  static getStatusText(status: Import['status']): string {
    switch (status) {
      case 'pending':
        return 'Pending';
      case 'running':
        return 'Running';
      case 'completed':
        return 'Completed';
      case 'failed':
        return 'Failed';
      default:
        return 'Unknown';
    }
  }

  /**
   * Get human-readable strategy text
   */
  static getStrategyText(strategy: string): string {
    switch (strategy) {
      case 'full_replace':
        return 'Full Replace';
      case 'merge_add':
        return 'Merge Add';
      case 'merge_update':
        return 'Merge Update';
      default:
        return strategy;
    }
  }

  /**
   * Format file size in human-readable format
   */
  static formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  /**
   * Format date for display
   */
  static formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
  }

  /**
   * Calculate restore duration
   */
  static calculateDuration(startDate?: string, endDate?: string): string {
    if (!startDate || !endDate) return 'N/A';
    
    const start = new Date(startDate);
    const end = new Date(endDate);
    const diffMs = end.getTime() - start.getTime();
    
    const seconds = Math.floor(diffMs / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
      return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds % 60}s`;
    } else {
      return `${seconds}s`;
    }
  }

  /**
   * Get restore progress percentage (for running restores)
   */
  static getProgress(restore: Import): number {
    if (restore.status === 'completed') return 100;
    if (restore.status === 'failed') return 0;
    if (restore.status === 'pending') return 0;
    
    // For running restores, we can't determine exact progress without additional data
    // Return indeterminate progress
    return -1;
  }
}
