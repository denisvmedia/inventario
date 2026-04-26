<script setup lang="ts">
/**
 * FileEditView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Edits standalone file metadata (filename, title, description, tags) plus
 * the optional commodity / location linking with grouped Reka UI selects.
 * Export-linked files render the linkage section as read-only because the
 * backend forbids manual relinks.
 *
 * Legacy DOM anchors (`.file-edit`, `.breadcrumb-link`, `#path`, `#title`,
 * `#description`, `#tags`, `#linked_entity_type`, `#linked_entity_id`,
 * `#linked_entity_meta`, `.file-extension`, `.tags-preview`, `.tag`,
 * `.tag-remove`) are preserved as no-op markers so existing Playwright
 * selectors keep resolving — see devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { ArrowLeft, ExternalLink, Link as LinkIcon, X } from 'lucide-vue-next'

import fileService, { type FileEntity } from '@/services/fileService'
import commodityService from '@/services/commodityService'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import { useGroupStore } from '@/stores/groupStore'
import {
  is404Error as checkIs404Error,
  get404Message,
  get404Title,
  getErrorMessage,
} from '@/utils/errorUtils'
import ResourceNotFound from '@/components/ResourceNotFound.vue'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Textarea } from '@design/ui/textarea'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@design/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import Banner from '@design/patterns/Banner.vue'
import FormFooter from '@design/patterns/FormFooter.vue'
import FormSection from '@design/patterns/FormSection.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import {
  fileEditFormSchema,
  defaultFileEditFormValues,
  type FileEditFormInput,
} from './FileForm.schema'

type AnyRecord = Record<string, unknown>
type CommodityOption = { label: string; value: string }
type CommodityGroup = { label: string; items: CommodityOption[] }
type LocationOption = { label: string; value: string }

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const ENTITY_TYPE_OPTIONS = [
  { label: 'No link (standalone file)', value: '__none__' },
  { label: 'Commodity', value: 'commodity' },
  { label: 'Location', value: 'location' },
] as const

const COMMODITY_META_OPTIONS = [
  { label: 'Images', value: 'images' },
  { label: 'Invoices', value: 'invoices' },
  { label: 'Manuals', value: 'manuals' },
] as const

const LOCATION_META_OPTIONS = [
  { label: 'Images', value: 'images' },
  { label: 'Files', value: 'files' },
] as const

// Sentinel used by Reka Select because empty-string values are not
// allowed by the underlying Listbox primitive.
const NONE = '__none__'

const file = ref<FileEntity | null>(null)
const loading = ref<boolean>(true)
const loadError = ref<string | null>(null)
const lastError = ref<unknown>(null)
const is404 = computed<boolean>(() => !!lastError.value && checkIs404Error(lastError.value as never))
const saveError = ref<string | null>(null)

const fileId = computed<string>(() => route.params.id as string)
const isExportFile = computed<boolean>(() => file.value?.linked_entity_type === 'export')

const loadingCommodities = ref<boolean>(false)
const commodityGroups = ref<CommodityGroup[]>([])
const loadingLocations = ref<boolean>(false)
const locationOptions = ref<LocationOption[]>([])

const { handleSubmit, isSubmitting, setFieldValue, setValues, values } =
  useForm<FileEditFormInput>({
    validationSchema: toTypedSchema(fileEditFormSchema),
    initialValues: defaultFileEditFormValues(),
  })

const tagsInput = ref<string>('')

const backLinkText = computed<string>(() =>
  route.query.from === 'export' ? 'Back to Export File' : 'Back to File',
)

function syncTagsFromInput() {
  const tags = tagsInput.value
    .split(',')
    .map((tag) => tag.trim())
    .filter((tag) => tag.length > 0)
  setFieldValue('tags', Array.from(new Set(tags)))
}

function removeTag(tag: string) {
  const next = (values.tags ?? []).filter((t) => t !== tag)
  setFieldValue('tags', next)
  tagsInput.value = next.join(', ')
}

function isLinked(f: FileEntity): boolean {
  return fileService.isLinked(f)
}
function getLinkedEntityDisplay(f: FileEntity): string {
  return fileService.getLinkedEntityDisplay(f)
}
function getLinkedEntityUrl(f: FileEntity): string {
  return fileService.getLinkedEntityUrl(f, route)
}

async function loadCommodities() {
  if (commodityGroups.value.length > 0 || loadingCommodities.value) return
  loadingCommodities.value = true
  try {
    const [locationsResp, areasResp, commoditiesResp] = await Promise.all([
      locationService.getLocations(),
      areaService.getAreas(),
      commodityService.getCommodities(),
    ])
    const locationMap = new Map<string, AnyRecord>(
      (locationsResp.data.data as { id: string; attributes: AnyRecord }[]).map((l) => [
        l.id,
        l.attributes,
      ]),
    )
    const areaMap = new Map<string, AnyRecord & { id: string }>(
      (areasResp.data.data as { id: string; attributes: AnyRecord }[]).map((a) => [
        a.id,
        { ...a.attributes, id: a.id },
      ]),
    )
    const groups = new Map<string, CommodityGroup>()
    for (const c of commoditiesResp.data.data as { id: string; attributes: AnyRecord }[]) {
      const areaId = c.attributes.area_id as string | undefined
      if (!areaId) continue
      const area = areaMap.get(areaId)
      if (!area) continue
      const location = locationMap.get(area.location_id as string)
      if (!location) continue
      const groupKey = `${location.id ?? ''}-${area.id}`
      if (!groups.has(groupKey)) {
        groups.set(groupKey, {
          label: `${location.name as string} - ${area.name as string}`,
          items: [],
        })
      }
      groups.get(groupKey)!.items.push({
        label: `${c.attributes.name as string} (${c.attributes.short_name as string})`,
        value: c.id,
      })
    }
    const sortedGroups = Array.from(groups.values())
      .filter((g) => g.items.length > 0)
      .sort((a, b) => a.label.localeCompare(b.label))
    sortedGroups.forEach((g) => g.items.sort((a, b) => a.label.localeCompare(b.label)))
    commodityGroups.value = sortedGroups
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load commodities'))
  } finally {
    loadingCommodities.value = false
  }
}

async function loadLocations() {
  if (locationOptions.value.length > 0 || loadingLocations.value) return
  loadingLocations.value = true
  try {
    const resp = await locationService.getLocations()
    const opts: LocationOption[] = (
      resp.data.data as { id: string; attributes: AnyRecord }[]
    )
      .map((l) => ({ label: l.attributes.name as string, value: l.id }))
      .sort((a, b) => a.label.localeCompare(b.label))
    locationOptions.value = opts
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to load locations'))
  } finally {
    loadingLocations.value = false
  }
}

async function loadFile() {
  loading.value = true
  loadError.value = null
  lastError.value = null
  try {
    const response = await fileService.getFile(fileId.value)
    const attrs = response.data.attributes as FileEntity
    file.value = attrs
    setValues(
      defaultFileEditFormValues({
        path: attrs.path ?? '',
        title: attrs.title ?? '',
        description: attrs.description ?? '',
        tags: [...(attrs.tags ?? [])],
        linked_entity_type: attrs.linked_entity_type ?? '',
        linked_entity_id: attrs.linked_entity_id ?? '',
        linked_entity_meta: attrs.linked_entity_meta ?? '',
      }),
    )
    tagsInput.value = (attrs.tags ?? []).join(', ')
    if (values.linked_entity_type === 'commodity' && values.linked_entity_id) {
      await loadCommodities()
    } else if (values.linked_entity_type === 'location' && values.linked_entity_id) {
      await loadLocations()
    }
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      loadError.value = getErrorMessage(err as never, 'file', 'Failed to load file')
    }
  } finally {
    loading.value = false
  }
}

function onEntityTypeChange(next: string) {
  const normalized = next === NONE ? '' : next
  setFieldValue('linked_entity_type', normalized)
  if (!isExportFile.value) {
    setFieldValue('linked_entity_id', '')
    setFieldValue('linked_entity_meta', '')
  }
  if (normalized === 'commodity') {
    void loadCommodities()
  } else if (normalized === 'location') {
    void loadLocations()
  }
}

const onSubmit = handleSubmit(async (formValues) => {
  saveError.value = null
  try {
    await fileService.updateFile(fileId.value, {
      title: formValues.title ?? '',
      description: formValues.description ?? '',
      tags: formValues.tags ?? [],
      path: formValues.path,
      linked_entity_type: formValues.linked_entity_type || undefined,
      linked_entity_id: formValues.linked_entity_id || undefined,
      linked_entity_meta: formValues.linked_entity_meta || undefined,
    })
    goBack()
  } catch (err) {
    saveError.value = getErrorMessage(err as never, 'file', 'Failed to save changes')
  }
})

function goBack() {
  const from = route.query.from as string | undefined
  const exportId = route.query.exportId as string | undefined
  if (from === 'export' && exportId) {
    router.push(groupStore.groupPath(`/files/${fileId.value}?from=export&exportId=${exportId}`))
  } else {
    router.push(groupStore.groupPath(`/files/${fileId.value}`))
  }
}

function goBackToList() {
  router.push(groupStore.groupPath('/files'))
}

watch(
  () => values.tags,
  (next) => {
    const joined = (next ?? []).join(', ')
    if (tagsInput.value.trim() !== joined.trim()) {
      tagsInput.value = joined
    }
  },
)

onMounted(loadFile)
</script>


<template>
  <PageContainer as="div" class="file-edit mx-auto max-w-3xl">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading file...</div>

    <ResourceNotFound
      v-else-if="is404"
      resource-type="file"
      :title="get404Title('file')"
      :message="get404Message('file')"
      go-back-text="Back to Files"
      @go-back="goBackToList"
      @try-again="loadFile"
    />

    <div
      v-else-if="loadError"
      class="error rounded-md border border-destructive/50 bg-destructive/10 p-4 text-destructive"
    >
      {{ loadError }}
    </div>

    <template v-else-if="file">
      <div class="mb-2 text-sm">
        <a
          href="#"
          class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
          @click.prevent="goBack"
        >
          <ArrowLeft class="size-4" aria-hidden="true" />
          <span>{{ backLinkText }}</span>
        </a>
      </div>

      <PageHeader title="Edit File" />

      <Banner v-if="saveError" variant="error" class="mb-4">{{ saveError }}</Banner>

      <!-- File preview / current-link summary -->
      <section class="file-preview-section mb-6 rounded-md border border-border bg-card p-4 shadow-sm">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="file-info min-w-0 flex-1">
            <h3 class="m-0 break-all text-lg font-semibold text-foreground">
              {{ file.path }}
            </h3>
            <div class="file-meta mt-2 flex flex-wrap gap-2 text-xs">
              <span class="file-type rounded-full bg-primary px-2 py-1 font-medium uppercase text-primary-foreground">
                {{ file.type }}
              </span>
              <span class="file-ext rounded-full bg-muted px-2 py-1 font-medium uppercase text-muted-foreground">
                {{ file.ext }}
              </span>
            </div>
            <div
              v-if="isLinked(file)"
              class="current-link-info mt-3 flex flex-wrap items-center gap-2 text-sm text-muted-foreground"
            >
              <LinkIcon class="size-4" aria-hidden="true" />
              <span>Currently linked to <strong>{{ getLinkedEntityDisplay(file) }}</strong></span>
              <a
                :href="getLinkedEntityUrl(file)"
                class="inline-flex items-center gap-1 text-primary hover:underline"
              >
                <ExternalLink class="size-4" aria-hidden="true" />
                View
              </a>
            </div>
          </div>
        </div>
      </section>

      <form
        class="flex flex-col gap-6 rounded-md border border-border bg-card p-6 shadow-sm"
        data-testid="file-edit-form"
        @submit="onSubmit"
      >
        <FormSection title="File Metadata">
          <!-- Filename + extension badge -->
          <FormField v-slot="{ componentField }" name="path">
            <FormItem id="path">
              <FormLabel required>Filename</FormLabel>
              <FormControl>
                <div class="filename-input-group flex items-stretch overflow-hidden rounded-md border border-border bg-background focus-within:ring-2 focus-within:ring-ring">
                  <Input
                    v-bind="componentField"
                    type="text"
                    placeholder="Enter filename (without extension)"
                    class="filename-input flex-1 rounded-none border-0 focus-visible:ring-0"
                  />
                  <span
                    v-if="file?.ext"
                    class="file-extension flex items-center border-l border-border bg-muted px-3 text-sm font-medium text-muted-foreground"
                  >
                    .{{ file.ext }}
                  </span>
                </div>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="title">
            <FormItem id="title">
              <FormLabel>Title</FormLabel>
              <FormControl>
                <Input
                  v-bind="componentField"
                  type="text"
                  placeholder="Optional display title (defaults to filename)"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="description">
            <FormItem id="description">
              <FormLabel>Description</FormLabel>
              <FormControl>
                <Textarea
                  v-bind="componentField"
                  rows="3"
                  placeholder="Optional description"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormItem id="tags">
            <FormLabel>Tags</FormLabel>
            <FormControl>
              <Input
                v-model="tagsInput"
                type="text"
                placeholder="Comma-separated tags"
                @blur="syncTagsFromInput"
                @change="syncTagsFromInput"
              />
            </FormControl>
            <p class="mt-1 text-xs text-muted-foreground">
              Separate tags with commas. Press Tab or click outside to apply.
            </p>
            <div
              v-if="(values.tags ?? []).length > 0"
              class="tags-preview mt-2 flex flex-wrap gap-1.5"
            >
              <span
                v-for="tag in values.tags"
                :key="tag"
                class="tag inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
              >
                {{ tag }}
                <button
                  type="button"
                  class="tag-remove inline-flex size-4 items-center justify-center rounded-full hover:bg-foreground/10"
                  :aria-label="`Remove tag ${tag}`"
                  @click="removeTag(tag)"
                >
                  <X class="size-3" aria-hidden="true" />
                </button>
              </span>
            </div>
          </FormItem>
        </FormSection>

        <FormSection v-if="file" title="File Details" class="text-sm">
          <FormItem>
            <FormLabel>MIME Type</FormLabel>
            <FormControl>
              <div class="readonly-display w-full break-all rounded-md border border-border bg-muted/50 px-3 py-2 text-foreground">
                {{ file.mime_type || 'Unknown' }}
              </div>
            </FormControl>
          </FormItem>
          <FormItem>
            <FormLabel>Original Path</FormLabel>
            <FormControl>
              <div class="readonly-display file-path w-full break-all rounded-md border border-border bg-muted/50 px-3 py-2 text-foreground">
                {{ file.original_path || 'N/A' }}
              </div>
            </FormControl>
          </FormItem>
        </FormSection>

        <FormSection title="Linked Entity">
          <Banner v-if="isExportFile" variant="info">
            This file is linked to an export and cannot be unlinked or relinked manually.
          </Banner>

          <FormItem id="linked_entity_type">
            <FormLabel>Linked To</FormLabel>
            <FormControl>
              <Select
                :model-value="(values.linked_entity_type || NONE) as string"
                :disabled="isExportFile"
                @update:model-value="(v) => onEntityTypeChange(v as string)"
              >
                <SelectTrigger class="w-full">
                  <SelectValue placeholder="Select link type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="opt in ENTITY_TYPE_OPTIONS"
                    :key="opt.value"
                    :value="opt.value"
                  >
                    {{ opt.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FormControl>
          </FormItem>

          <!-- Commodity selector with grouping by Location/Area -->
          <FormField
            v-if="values.linked_entity_type === 'commodity'"
            v-slot="{ componentField, value }"
            name="linked_entity_id"
          >
            <FormItem id="linked_entity_id">
              <FormLabel required>Commodity</FormLabel>
              <FormControl>
                <Select
                  :model-value="(value as string | undefined)"
                  :disabled="isExportFile || loadingCommodities"
                  @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
                >
                  <SelectTrigger class="w-full">
                    <SelectValue
                      :placeholder="loadingCommodities ? 'Loading commodities...' : 'Select a commodity'"
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup
                      v-for="group in commodityGroups"
                      :key="group.label"
                    >
                      <SelectLabel>{{ group.label }}</SelectLabel>
                      <SelectItem
                        v-for="item in group.items"
                        :key="item.value"
                        :value="item.value"
                      >
                        {{ item.label }}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <!-- Location selector -->
          <FormField
            v-if="values.linked_entity_type === 'location'"
            v-slot="{ componentField, value }"
            name="linked_entity_id"
          >
            <FormItem id="linked_entity_id">
              <FormLabel required>Location</FormLabel>
              <FormControl>
                <Select
                  :model-value="(value as string | undefined)"
                  :disabled="isExportFile || loadingLocations"
                  @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
                >
                  <SelectTrigger class="w-full">
                    <SelectValue
                      :placeholder="loadingLocations ? 'Loading locations...' : 'Select a location'"
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="loc in locationOptions"
                      :key="loc.value"
                      :value="loc.value"
                    >
                      {{ loc.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <!-- Meta selector (image/manual/invoice/etc.) -->
          <FormField
            v-if="values.linked_entity_type === 'commodity' || values.linked_entity_type === 'location'"
            v-slot="{ componentField, value }"
            name="linked_entity_meta"
          >
            <FormItem id="linked_entity_meta">
              <FormLabel>Category</FormLabel>
              <FormControl>
                <Select
                  :model-value="(value as string | undefined)"
                  :disabled="isExportFile"
                  @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
                >
                  <SelectTrigger class="w-full">
                    <SelectValue placeholder="Select category" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="opt in (values.linked_entity_type === 'commodity' ? COMMODITY_META_OPTIONS : LOCATION_META_OPTIONS)"
                      :key="opt.value"
                      :value="opt.value"
                    >
                      {{ opt.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>
        </FormSection>

        <FormFooter>
          <Button type="button" variant="outline" @click="goBack">Cancel</Button>
          <Button type="submit" :disabled="isSubmitting">
            {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
          </Button>
        </FormFooter>
      </form>
    </template>
  </PageContainer>
</template>
