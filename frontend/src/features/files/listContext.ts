// The Files list as a list context (lib/list-context.ts): how its URL turns
// into a query, and how a file opened from it finds its way back and to its
// neighbors. FilesListPage and the file review page both read the list URL
// through `parseFilesListParams`, so they ask the server the same question
// under the same query key and share the cached pages.
import { listPageQuery, type ListPageSource } from "@/hooks/useListNavigation"
import {
  listContextPage,
  listContextParams,
  withListContext,
  type ListContext,
} from "@/lib/list-context"

import { listFiles, type FileCategory, type ListFilesOptions } from "./api"
import { FILE_CATEGORIES, type FileCategoryTile } from "./constants"
import { fileKeys } from "./keys"

export const FILES_PAGE_SIZE = 24

export interface FilesListParams {
  tile: FileCategoryTile
  search: string
  tags: string[]
  page: number
}

export function parseFilesListParams(params: URLSearchParams): FilesListParams {
  return {
    tile: parseTileParam(params.get("category")),
    search: params.get("q") ?? "",
    tags: splitTags(params.get("tags")),
    page: parsePageParam(params.get("page")),
  }
}

/** The list request for a parsed Files list URL. */
export function filesListOptions(params: FilesListParams): ListFilesOptions {
  return {
    page: params.page,
    perPage: FILES_PAGE_SIZE,
    // `tile` is validated against the closed set in parseTileParam, so every
    // branch other than "all" is a FileCategory.
    category: params.tile === "all" ? undefined : (params.tile as FileCategory),
    search: params.search || undefined,
    tags: params.tags.length ? params.tags : undefined,
  }
}

export function filesListPath(slug: string): string {
  return `/g/${encodeURIComponent(slug)}/files`
}

/** True when the context is the group's Files list, the one a file can walk. */
export function isFilesListContext(ctx: ListContext, slug: string): boolean {
  return !!slug && ctx.pathname === filesListPath(slug)
}

/** The Files list behind a context, as a source for useListNavigation. */
export function filesListSource(ctx: ListContext, slug: string): ListPageSource {
  const base = parseFilesListParams(listContextParams(ctx))
  return {
    pageSize: FILES_PAGE_SIZE,
    page: listContextPage(ctx),
    pageQuery: (page) => {
      const opts = filesListOptions({ ...base, page })
      return listPageQuery({
        queryKey: fileKeys.list(slug, opts),
        queryFn: ({ signal }) => listFiles({ ...opts, signal }),
        select: (data) => ({ ids: data.files.map((f) => f.file.id), total: data.total }),
      })
    },
  }
}

/**
 * Where a file opened from the Files list returns to: the same list with the
 * file's detail panel open, as the user left it.
 */
export function fileInListHref(ctx: ListContext, fileId: string): string {
  const path = `${ctx.pathname}/${encodeURIComponent(fileId)}`
  return ctx.search ? `${path}?${ctx.search}` : path
}

/** The review page of a file, carrying the list it was opened from. */
export function fileReviewHref(slug: string, fileId: string, ctx: ListContext | null): string {
  return withListContext(`${filesListPath(slug)}/${encodeURIComponent(fileId)}/edit`, ctx)
}

function splitTags(raw: string | null): string[] {
  if (!raw) return []
  return raw
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
}

// URL params are untrusted: a typo or a hand-crafted link with a junk
// `category=warranty` would otherwise reach the BE as an invalid filter
// (400) and leave every tile unselected. Fall back to `"all"` for any
// value that isn't part of the closed enum.
function parseTileParam(raw: string | null): FileCategoryTile {
  if (!raw) return "all"
  return (FILE_CATEGORIES as readonly string[]).includes(raw) ? (raw as FileCategoryTile) : "all"
}

// Same defensiveness for `?page=`. `Number("abc")` is NaN, which would
// poison the list query key and the pagination UI; `parseInt` + a
// finite-and-positive guard collapses any garbage to page 1.
function parsePageParam(raw: string | null): number {
  if (!raw) return 1
  const parsed = Number.parseInt(raw, 10)
  if (!Number.isFinite(parsed) || parsed < 1) return 1
  return parsed
}
