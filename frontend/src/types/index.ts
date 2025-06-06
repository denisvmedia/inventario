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

export type ExportType = 'full_database' | 'selected_items' | 'locations' | 'areas' | 'commodities';

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
  file_path: string;
  created_date: string;
  completed_date?: string;
  error_message?: string;
  description: string;
}
