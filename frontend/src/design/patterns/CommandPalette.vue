<script setup lang="ts">
/**
 * CommandPalette — global Cmd+K / Ctrl+K search dialog (#1330 PR 5.4).
 *
 * Composes shadcn `<Dialog>` (focus trap + escape + return focus from
 * Reka UI) with a debounced search input, server-side queries against
 * `/api/v1/search` (commodities + files via the existing backend
 * fallback), and keyboard navigation across the result list (Up/Down
 * arrows, Enter to open, Esc to close).
 *
 * The pattern is presentation-only — the parent (App.vue) registers
 * the global Cmd+K hotkey via `useKeyboardShortcuts` and toggles the
 * `open` v-model. Tests can drive the same v-model.
 */
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  Box,
  CornerDownLeft,
  FileText,
  Loader2,
  Search,
} from 'lucide-vue-next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@design/ui/dialog'
import { useDebouncedSearch } from '@design/composables/useDebouncedSearch'

import searchService, {
  type CommoditySearchAttrs,
  type FileSearchAttrs,
  type SearchResource,
} from '@/services/searchService'
import { useGroupStore } from '@/stores/groupStore'

interface PaletteResult {
  id: string
  kind: 'commodity' | 'file'
  title: string
  subtitle?: string
  to: string
}

const open = defineModel<boolean>('open', { default: false })

const router = useRouter()
const groupStore = useGroupStore()

const loading = ref(false)
const error = ref<string | null>(null)
const results = ref<PaletteResult[]>([])
const activeIndex = ref(0)

const { query, debouncedQuery } = useDebouncedSearch({
  delay: 300,
  minLength: 2,
  onSearch: (q) => runSearch(q),
})

const showInitialHint = computed(
  () => !loading.value && results.value.length === 0 && !debouncedQuery.value,
)

async function runSearch(q: string) {
  loading.value = true
  error.value = null
  try {
    const [commResp, fileResp] = await Promise.all([
      searchService.search<CommoditySearchAttrs>(q, { type: 'commodities', limit: 8 }),
      searchService.search<FileSearchAttrs>(q, { type: 'files', limit: 8 }),
    ])

    const commodityResults: PaletteResult[] = commResp.data.map(
      (r: SearchResource<CommoditySearchAttrs>) => ({
        id: r.id,
        kind: 'commodity',
        title: r.attributes.name,
        subtitle: r.attributes.short_name || undefined,
        to: groupStore.groupPath(`/commodities/${r.id}`),
      }),
    )
    const fileResults: PaletteResult[] = fileResp.data.map(
      (r: SearchResource<FileSearchAttrs>) => ({
        id: r.id,
        kind: 'file',
        title: r.attributes.title || r.attributes.path || 'Untitled',
        subtitle: r.attributes.description || undefined,
        to: groupStore.groupPath(`/files/${r.id}`),
      }),
    )

    results.value = [...commodityResults, ...fileResults]
    activeIndex.value = 0
  } catch (err: any) {
    error.value = err?.response?.data?.message ?? err?.message ?? 'Search failed.'
    results.value = []
  } finally {
    loading.value = false
  }
}

const commodityResults = computed(() =>
  results.value.filter((r) => r.kind === 'commodity'),
)
const fileResults = computed(() => results.value.filter((r) => r.kind === 'file'))

function selectResult(result: PaletteResult) {
  open.value = false
  router.push(result.to)
}

function move(delta: number) {
  if (results.value.length === 0) return
  const next = activeIndex.value + delta
  if (next < 0) activeIndex.value = results.value.length - 1
  else if (next >= results.value.length) activeIndex.value = 0
  else activeIndex.value = next
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    move(1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    move(-1)
  } else if (event.key === 'Enter') {
    const selected = results.value[activeIndex.value]
    if (selected) {
      event.preventDefault()
      selectResult(selected)
    }
  }
}

// Reset search state when the dialog opens / closes so the next
// invocation starts from a clean slate.
watch(open, async (isOpen) => {
  if (!isOpen) {
    query.value = ''
    results.value = []
    activeIndex.value = 0
    error.value = null
    loading.value = false
  } else {
    await nextTick()
  }
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      data-testid="command-palette"
      class="overflow-hidden p-0 sm:max-w-xl"
      @keydown="onKeydown"
    >
      <DialogHeader class="sr-only">
        <DialogTitle>Search</DialogTitle>
        <DialogDescription>
          Find commodities and files by name. Use the arrow keys to navigate
          and Enter to open.
        </DialogDescription>
      </DialogHeader>

      <div class="flex items-center gap-2 border-b border-border px-3">
        <Search class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <input
          v-model="query"
          type="search"
          role="searchbox"
          aria-label="Search commodities and files"
          placeholder="Search commodities, files…"
          class="h-12 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          autofocus
        />
        <Loader2
          v-if="loading"
          class="size-4 shrink-0 animate-spin text-muted-foreground"
          aria-hidden="true"
        />
      </div>

      <div class="max-h-96 overflow-y-auto py-2">
        <p
          v-if="showInitialHint"
          class="px-4 py-6 text-center text-sm text-muted-foreground"
        >
          Type at least 2 characters to search.
        </p>

        <p
          v-else-if="error"
          class="px-4 py-6 text-center text-sm text-destructive"
          role="alert"
        >
          {{ error }}
        </p>

        <p
          v-else-if="!loading && results.length === 0 && debouncedQuery"
          class="px-4 py-6 text-center text-sm text-muted-foreground"
        >
          No results for "{{ debouncedQuery }}".
        </p>

        <div v-else class="flex flex-col gap-2">
          <section v-if="commodityResults.length > 0">
            <h3 class="px-4 py-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Commodities
            </h3>
            <ul role="listbox" aria-label="Commodity results">
              <li v-for="(r, i) in commodityResults" :key="r.id">
                <button
                  type="button"
                  :class="[
                    'flex w-full items-center gap-3 px-4 py-2 text-left text-sm motion-safe:transition-colors',
                    activeIndex === results.indexOf(r)
                      ? 'bg-accent text-accent-foreground'
                      : 'hover:bg-accent/50',
                  ]"
                  :aria-selected="activeIndex === results.indexOf(r)"
                  @mouseenter="activeIndex = results.indexOf(r)"
                  @click="selectResult(r)"
                >
                  <Box class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span class="min-w-0 flex-1 truncate">{{ r.title }}</span>
                  <span
                    v-if="r.subtitle"
                    class="shrink-0 text-xs text-muted-foreground"
                  >
                    {{ r.subtitle }}
                  </span>
                  <CornerDownLeft
                    v-if="activeIndex === results.indexOf(r)"
                    class="size-3 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </button>
                <template v-if="i === commodityResults.length - 1 && fileResults.length > 0">
                  <hr class="my-2 border-border" />
                </template>
              </li>
            </ul>
          </section>

          <section v-if="fileResults.length > 0">
            <h3 class="px-4 py-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Files
            </h3>
            <ul role="listbox" aria-label="File results">
              <li v-for="r in fileResults" :key="r.id">
                <button
                  type="button"
                  :class="[
                    'flex w-full items-center gap-3 px-4 py-2 text-left text-sm motion-safe:transition-colors',
                    activeIndex === results.indexOf(r)
                      ? 'bg-accent text-accent-foreground'
                      : 'hover:bg-accent/50',
                  ]"
                  :aria-selected="activeIndex === results.indexOf(r)"
                  @mouseenter="activeIndex = results.indexOf(r)"
                  @click="selectResult(r)"
                >
                  <FileText class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span class="min-w-0 flex-1 truncate">{{ r.title }}</span>
                  <span
                    v-if="r.subtitle"
                    class="shrink-0 text-xs text-muted-foreground"
                  >
                    {{ r.subtitle }}
                  </span>
                  <CornerDownLeft
                    v-if="activeIndex === results.indexOf(r)"
                    class="size-3 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </button>
              </li>
            </ul>
          </section>
        </div>
      </div>

      <footer
        class="flex items-center justify-between gap-2 border-t border-border bg-muted/40 px-4 py-2 text-xs text-muted-foreground"
      >
        <span class="flex items-center gap-3">
          <span><kbd class="rounded border border-border bg-background px-1.5 py-0.5">↑↓</kbd> navigate</span>
          <span><kbd class="rounded border border-border bg-background px-1.5 py-0.5">↵</kbd> open</span>
          <span><kbd class="rounded border border-border bg-background px-1.5 py-0.5">esc</kbd> close</span>
        </span>
        <span class="hidden sm:inline">Search powered by /api/v1/search.</span>
      </footer>
    </DialogContent>
  </Dialog>
</template>
