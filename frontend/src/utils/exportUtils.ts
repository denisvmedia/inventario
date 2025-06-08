import type { Export } from '@/types'

/**
 * Check if an export has been soft deleted
 */
export function isExportDeleted(exportItem: Export): boolean {
  return !!exportItem.deleted_at
}

/**
 * Check if operations can be performed on an export
 */
export function canPerformOperations(exportItem: Export): boolean {
  return !isExportDeleted(exportItem)
}

/**
 * Get the display status for an export including deleted state
 */
export function getExportDisplayStatus(exportItem: Export): string {
  if (isExportDeleted(exportItem)) {
    return 'Deleted'
  }
  
  switch (exportItem.status) {
    case 'pending':
      return 'Pending'
    case 'in_progress':
      return 'In Progress'
    case 'completed':
      return 'Completed'
    case 'failed':
      return 'Failed'
    default:
      return 'Unknown'
  }
}

/**
 * Get CSS classes for export status styling
 */
export function getExportStatusClasses(exportItem: Export): string {
  if (isExportDeleted(exportItem)) {
    return 'export-status export-status--deleted'
  }

  switch (exportItem.status) {
    case 'pending':
      return 'export-status export-status--pending'
    case 'in_progress':
      return 'export-status export-status--in-progress'
    case 'completed':
      return 'export-status export-status--completed'
    case 'failed':
      return 'export-status export-status--failed'
    default:
      return 'export-status export-status--unknown'
  }
}

/**
 * Check if an export can be downloaded
 */
export function canDownloadExport(exportItem: Export): boolean {
  return exportItem.status === 'completed' && canPerformOperations(exportItem)
}

/**
 * Check if an export can be retried
 */
export function canRetryExport(exportItem: Export): boolean {
  return exportItem.status === 'failed' && canPerformOperations(exportItem)
}

/**
 * Get a human-readable description of the export's current state
 */
export function getExportStateDescription(exportItem: Export): string {
  if (isExportDeleted(exportItem)) {
    return 'This export has been deleted and will be cleaned up automatically.'
  }

  switch (exportItem.status) {
    case 'pending':
      return 'Export is queued for processing.'
    case 'in_progress':
      return 'Export is currently being processed.'
    case 'completed':
      return 'Export completed successfully and is ready for download.'
    case 'failed':
      return 'Export failed. You can retry the export or check the error details.'
    default:
      return 'Export status is unknown.'
  }
}

/**
 * Format file size in human-readable format
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 Bytes'

  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
