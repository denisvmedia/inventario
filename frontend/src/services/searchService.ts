import api from './api'

const API_URL = '/api/v1/search'

export type SearchEntityType = 'commodities' | 'files'

export interface SearchResource<TAttrs = Record<string, unknown>> {
  id: string
  type: string
  attributes: TAttrs
}

export interface CommoditySearchAttrs {
  name: string
  short_name?: string
  type?: string
  status?: string
  area_id?: string
  draft?: boolean
}

export interface FileSearchAttrs {
  title?: string
  path?: string
  type?: string
  ext?: string
  description?: string
}

export interface SearchResponse<TAttrs> {
  data: SearchResource<TAttrs>[]
  meta?: { total?: number }
}

export interface SearchOptions {
  type?: SearchEntityType
  limit?: number
  offset?: number
}

const searchService = {
  /**
   * Search the backend's `/api/v1/search` endpoint. The current backend
   * fallback (`go/apiserver/search.go:searchWithBasicFallback`) ships
   * results for `commodities` and `files`; other entity types return
   * 501 today, so the CommandPalette only queries those two for now.
   */
  async search<T = Record<string, unknown>>(
    query: string,
    options: SearchOptions = {},
  ): Promise<SearchResponse<T>> {
    const trimmed = query.trim()
    if (!trimmed) return { data: [] }

    const params: Record<string, unknown> = {
      q: trimmed,
      type: options.type ?? 'commodities',
    }
    if (options.limit !== undefined) params.limit = options.limit
    if (options.offset !== undefined) params.offset = options.offset

    const response = await api.get(API_URL, { params })
    const body = response.data
    return {
      data: Array.isArray(body?.data) ? body.data : [],
      meta: body?.meta,
    }
  },
}

export default searchService
