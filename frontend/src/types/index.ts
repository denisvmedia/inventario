export interface ApiResponse<T> {
  data: T;
  meta?: Record<string, any>;
}

export interface ApiError {
  errors: {
    status: string;
    title: string;
    detail: string;
  }[];
}

export interface ResourceObject<T> {
  id: string;
  type: string;
  attributes: T;
}

export interface Area {
  name: string;
}

export interface Location {
  name: string;
  areas: string[];
}

export interface Commodity {
  name: string;
  description?: string;
  location_id: string;
  area_id?: string;
  quantity?: number;
  price?: number;
  currency?: string;
  purchase_date?: string;
  expiry_date?: string;
  tags?: string[];
}

export interface Image {
  id: string;
  name: string;
  content_type: string;
  size: number;
  url: string;
}

export interface Manual {
  id: string;
  name: string;
  content_type: string;
  size: number;
  url: string;
}

export interface Invoice {
  id: string;
  name: string;
  content_type: string;
  size: number;
  url: string;
}

export type ExportStatus = 'pending' | 'in_progress' | 'completed' | 'failed';

export type ExportType = 'full_database' | 'selected_items' | 'locations' | 'areas' | 'commodities' | 'imported';

export type ExportSelectedItemType = 'location' | 'area' | 'commodity';

export interface ExportSelectedItem {
  id: string;
  type: ExportSelectedItemType;
  name?: string;
  include_all?: boolean;
  location_id?: string; // For areas: which location they belong to
  area_id?: string;     // For commodities: which area they belong to
}

export interface Export {
  id?: string;
  type: ExportType;
  status: ExportStatus;
  include_file_data: boolean;
  selected_items: ExportSelectedItem[];
  file_id?: string;
  file_path?: string; // Deprecated: will be removed after migration
  created_date: string;
  completed_date?: string;
  deleted_at?: string;
  error_message?: string;
  description: string;
  // Export statistics
  file_size?: number;
  location_count?: number;
  area_count?: number;
  commodity_count?: number;
  image_count?: number;
  invoice_count?: number;
  manual_count?: number;
  binary_data_size?: number;
}

export interface RestoreOptions {
  strategy: string;
  include_file_data: boolean;
  dry_run: boolean;
}

export type RestoreStatus = 'pending' | 'running' | 'completed' | 'failed';

export interface RestoreRequest {
  description: string;
  source_file_path: string;
  options: RestoreOptions;
}

export type RestoreStepResult = 'todo' | 'in_progress' | 'success' | 'error' | 'skipped';

export interface RestoreStep {
  id: string;
  name: string;
  result: RestoreStepResult;
  duration?: number;
  reason?: string;
  created_date: string;
  updated_date: string;
}

export interface RestoreOperation {
  id: string;
  export_id: string;
  description: string;
  status: RestoreStatus;
  options: RestoreOptions;
  steps: RestoreStep[];
  created_date: string;
  started_date?: string;
  completed_date?: string;
  error_message?: string;
}
