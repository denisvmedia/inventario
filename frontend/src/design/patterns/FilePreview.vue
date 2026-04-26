<script setup lang="ts">
/**
 * FilePreview — grid-card representation of a single file entity.
 *
 * Consumed by FileListView (#1328 PR 3.3) and ready for re-use in any
 * other gallery view that lands in later phases. The pattern is
 * presentation-only: thumbnails are rendered from a parent-supplied
 * signed URL, broken images bubble up via the `imageError` emit so the
 * parent (which owns fileService) can refresh the URL, and the linked-
 * entity badge is a `<router-link>` driven by a parent-computed
 * `{ display, url, icon }` triple.
 *
 * Class anchors: `.file-card` stays on the outermost element so the
 * existing Playwright suite that targets file grids by CSS class keeps
 * working through the migration window. `.file-item`, `.file-preview`,
 * `.preview-image`, `.file-info`, `.file-name-text`, and `.file-actions`
 * are preserved for commodity / location file-upload E2E coverage.
 */
import type { FunctionalComponent, HTMLAttributes } from 'vue'
import { computed } from 'vue'
import {
  Archive,
  Box,
  CircleHelp,
  Download,
  ExternalLink,
  File as FileIcon,
  FileOutput,
  FileText,
  Image as ImageIcon,
  Link as LinkIcon,
  Lock,
  type LucideProps,
  MapPin,
  Music,
  Pencil,
  Trash2,
  Video,
} from 'lucide-vue-next'
import { RouterLink } from 'vue-router'

import type { FileEntity } from '@/services/fileService'
import { cn } from '@design/lib/utils'

import IconButton from './IconButton.vue'

type EntityIconName = 'commodity' | 'location' | 'export' | 'link'

type LinkedEntity = {
  display: string
  url: string
  icon: EntityIconName
}

type Props = {
  file: FileEntity
  /** Pre-resolved signed URL for image thumbnails. */
  thumbnailUrl?: string
  /** Optional badge linking back to the entity that owns this file. */
  linkedEntity?: LinkedEntity
  /** When false, the trash button is replaced with a disabled lock icon. */
  canDelete?: boolean
  /** Tooltip / title text for the disabled lock state. */
  deleteRestrictionReason?: string
  showDetailsAction?: boolean
  testId?: string
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  canDelete: true,
})

type Emits = {
  view: []
  download: []
  edit: []
  details: []
  delete: []
  imageError: [event: Event]
}
const emit = defineEmits<Emits>()

const fileTypeLabels: Record<string, string> = {
  image: 'Image',
  document: 'Document',
  video: 'Video',
  audio: 'Audio',
  archive: 'Archive',
  other: 'Other',
}

const fileTypeIcons: Record<string, FunctionalComponent<LucideProps>> = {
  image: ImageIcon,
  document: FileText,
  video: Video,
  audio: Music,
  archive: Archive,
  other: FileIcon,
}

const entityIcons: Record<EntityIconName, FunctionalComponent<LucideProps>> = {
  commodity: Box,
  location: MapPin,
  export: FileOutput,
  link: LinkIcon,
}

const displayTitle = computed(() => {
  const file = props.file
  if (file.title?.trim()) return file.title
  if (file.path?.trim()) return file.path
  return 'Untitled'
})

const fileTypeLabel = computed(
  () => fileTypeLabels[props.file.type] ?? props.file.type,
)

const fileTypeIcon = computed<FunctionalComponent<LucideProps>>(() => {
  if (props.file.type === 'document' && props.file.mime_type === 'application/pdf') {
    return FileText
  }
  return fileTypeIcons[props.file.type] ?? FileIcon
})

const entityIcon = computed(() =>
  props.linkedEntity ? entityIcons[props.linkedEntity.icon] : null,
)

const tags = computed(() => props.file.tags ?? [])
const visibleTags = computed(() => tags.value.slice(0, 3))
const overflowTags = computed(() => Math.max(0, tags.value.length - 3))

function onCardClick() {
  emit('view')
}
function onCardKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('view')
  }
}

function onImgError(event: Event) {
  emit('imageError', event)
}
</script>

<template>
  <div
    role="button"
    tabindex="0"
    :data-testid="testId"
    :data-file-id="file.id"
    :class="
      cn(
        'file-card file-item group relative flex flex-col overflow-hidden rounded-md border border-border bg-card shadow-sm',
        'cursor-pointer motion-safe:transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        props.class,
      )
    "
    @click="onCardClick"
    @keydown="onCardKeydown"
  >
    <div class="file-preview flex h-40 items-center justify-center bg-muted">
      <img
        v-if="file.type === 'image' && thumbnailUrl"
        :src="thumbnailUrl"
        :alt="displayTitle"
        :data-file-id="file.id"
        class="preview-image h-full w-full object-cover"
        @error="onImgError"
      />
      <component
        :is="fileTypeIcon"
        v-else
        class="file-icon size-12 text-muted-foreground"
        aria-hidden="true"
      />
    </div>

    <div class="file-info flex flex-col gap-2 p-4">
      <h3
        :title="displayTitle"
        class="file-name truncate text-sm font-semibold text-foreground"
      >
        <span class="file-name-text">{{ displayTitle }}</span>
      </h3>
      <p
        :title="file.description"
        class="truncate text-sm text-muted-foreground"
      >
        {{ file.description || 'No description' }}
      </p>

      <div class="flex flex-wrap gap-1">
        <span
          class="inline-flex items-center rounded border border-border bg-muted px-2 py-0.5 text-xs text-muted-foreground"
        >
          {{ fileTypeLabel }}
        </span>
        <span
          class="inline-flex items-center rounded border border-border bg-muted px-2 py-0.5 text-xs text-muted-foreground"
        >
          {{ file.ext }}
        </span>
      </div>

      <div v-if="visibleTags.length > 0" class="flex flex-wrap items-center gap-1">
        <span
          v-for="tag in visibleTags"
          :key="tag"
          class="inline-flex items-center rounded-full bg-primary px-2 py-0.5 text-xs font-medium text-primary-foreground"
        >
          {{ tag }}
        </span>
        <span
          v-if="overflowTags > 0"
          class="text-xs text-muted-foreground"
        >
          +{{ overflowTags }} more
        </span>
      </div>

      <RouterLink
        v-if="linkedEntity"
        :to="linkedEntity.url"
        title="View linked entity"
        class="inline-flex max-w-full items-center gap-1.5 self-start rounded-md border border-blue-200 bg-blue-50 px-2 py-1 text-xs font-medium text-blue-900 hover:border-blue-400 hover:bg-blue-100 dark:border-blue-900/50 dark:bg-blue-950/40 dark:text-blue-100"
        @click.stop
      >
        <component
          :is="entityIcon"
          v-if="entityIcon"
          class="size-3 shrink-0"
          aria-hidden="true"
        />
        <span class="truncate">{{ linkedEntity.display }}</span>
        <ExternalLink class="size-3 shrink-0 opacity-70" aria-hidden="true" />
      </RouterLink>
    </div>

    <div
      class="file-actions absolute right-2 top-2 flex gap-1 opacity-0 motion-safe:transition-opacity group-hover:opacity-100 focus-within:opacity-100"
      @click.stop
    >
      <IconButton
        aria-label="Download file"
        title="Download"
        size="icon-sm"
        variant="secondary"
        class="btn btn-sm btn-primary"
        @click="emit('download')"
      >
        <Download />
      </IconButton>
      <IconButton
        v-if="showDetailsAction"
        aria-label="View file details"
        title="Details"
        size="icon-sm"
        variant="secondary"
        class="btn btn-sm btn-info"
        @click="emit('details')"
      >
        <CircleHelp />
      </IconButton>
      <IconButton
        aria-label="Edit file"
        title="Edit"
        size="icon-sm"
        variant="secondary"
        @click="emit('edit')"
      >
        <Pencil />
      </IconButton>
      <IconButton
        v-if="canDelete"
        aria-label="Delete file"
        title="Delete"
        size="icon-sm"
        variant="secondary"
        class="btn btn-sm btn-danger text-destructive hover:bg-destructive/10 hover:text-destructive"
        @click="emit('delete')"
      >
        <Trash2 />
      </IconButton>
      <IconButton
        v-else
        :aria-label="deleteRestrictionReason || 'Cannot delete'"
        :title="deleteRestrictionReason"
        size="icon-sm"
        variant="secondary"
        disabled
        class="text-muted-foreground"
      >
        <Lock />
      </IconButton>
    </div>
  </div>
</template>
