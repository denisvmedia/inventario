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