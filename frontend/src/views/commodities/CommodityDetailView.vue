<template>
  <div class="commodity-detail">
    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!commodity" class="not-found">Commodity not found</div>
    <div v-else>
      <div class="header">
        <h1>{{ commodity.attributes.name }}</h1>
        <div class="actions">
          <button class="btn btn-secondary" @click="editCommodity">Edit</button>
          <button class="btn btn-danger" @click="confirmDelete">Delete</button>
        </div>
      </div>

      <div class="commodity-info">
        <div class="info-card">
          <h2>Basic Information</h2>
          <div class="info-row">
            <span class="label">Short Name:</span>
            <span>{{ commodity.attributes.short_name }}</span>
          </div>
          <div class="info-row">
            <span class="label">Type:</span>
            <span>{{ getTypeName(commodity.attributes.type) }}</span>
          </div>
          <div class="info-row">
            <span class="label">Count:</span>
            <span>{{ commodity.attributes.count || 1 }}</span>
          </div>
          <div class="info-row">
            <span class="label">Status:</span>
            <span class="status" :class="commodity.attributes.status">
              {{ getStatusName(commodity.attributes.status) }}
            </span>
          </div>
          <div class="info-row">
            <span class="label">Purchase Date:</span>
            <span>{{ formatDate(commodity.attributes.purchase_date) }}</span>
          </div>
        </div>

        <div class="info-card">
          <h2>Price Information</h2>
          <div class="info-row">
            <span class="label">Original Price:</span>
            <span>{{ commodity.attributes.original_price }} {{ commodity.attributes.original_price_currency }}</span>
          </div>
          <div class="info-row">
            <span class="label">Converted Original Price:</span>
            <span>{{ commodity.attributes.converted_original_price }}</span>
          </div>
          <div class="info-row">
            <span class="label">Current Price:</span>
            <span>{{ commodity.attributes.current_price }}</span>
          </div>
        </div>

        <div class="info-card" v-if="commodity.attributes.serial_number || (commodity.attributes.extra_serial_numbers && commodity.attributes.extra_serial_numbers.length > 0) || (commodity.attributes.part_numbers && commodity.attributes.part_numbers.length > 0)">
          <h2>Serial Numbers and Part Numbers</h2>
          <div class="info-row" v-if="commodity.attributes.serial_number">
            <span class="label">Serial Number:</span>
            <span>{{ commodity.attributes.serial_number }}</span>
          </div>
          <div v-if="commodity.attributes.extra_serial_numbers && commodity.attributes.extra_serial_numbers.length > 0">
            <h3>Extra Serial Numbers</h3>
            <ul>
              <li v-for="(serial, index) in commodity.attributes.extra_serial_numbers" :key="index">
                {{ serial }}
              </li>
            </ul>
          </div>
          <div v-if="commodity.attributes.part_numbers && commodity.attributes.part_numbers.length > 0">
            <h3>Part Numbers</h3>
            <ul>
              <li v-for="(part, index) in commodity.attributes.part_numbers" :key="index">
                {{ part }}
              </li>
            </ul>
          </div>
        </div>

        <div class="info-card" v-if="commodity.attributes.tags && commodity.attributes.tags.length > 0">
          <h2>Tags</h2>
          <div class="tags">
            <span class="tag" v-for="(tag, index) in commodity.attributes.tags" :key="index">
              {{ tag }}
            </span>
          </div>
        </div>

        <div class="info-card" v-if="commodity.attributes.urls && commodity.attributes.urls.length > 0">
          <h2>URLs</h2>
          <ul>
            <li v-for="(url, index) in commodity.attributes.urls" :key="index">
              <a :href="url" target="_blank">{{ url }}</a>
            </li>
          </ul>
        </div>

        <!-- Images Section -->
        <div class="info-card full-width">
          <div class="section-header">
            <h2>Images</h2>
            <button class="btn btn-sm btn-primary" @click="showImageUploader = !showImageUploader">
              {{ showImageUploader ? 'Cancel' : 'Add Images' }}
            </button>
          </div>

          <FileUploader
            v-if="showImageUploader"
            :multiple="true"
            accept=".gif,.jpg,.jpeg,.png,.webp,image/gif,image/jpeg,image/png,image/webp"
            uploadPrompt="Drag and drop images here"
            @upload="uploadImages"
          />

          <div v-if="loadingImages" class="loading">Loading images...</div>
          <FileViewer
            v-else
            :files="images"
            fileType="images"
            :entityId="commodity.id"
            entityType="commodities"
            @delete="deleteImage"
            @update="updateImage"
            @download="downloadImage"
          />
        </div>

        <!-- Manuals Section -->
        <div class="info-card full-width">
          <div class="section-header">
            <h2>Manuals</h2>
            <button class="btn btn-sm btn-primary" @click="showManualUploader = !showManualUploader">
              {{ showManualUploader ? 'Cancel' : 'Add Manuals' }}
            </button>
          </div>

          <FileUploader
            v-if="showManualUploader"
            :multiple="true"
            accept=".pdf,.gif,.jpg,.jpeg,.png,.webp,application/pdf,image/gif,image/jpeg,image/png,image/webp"
            uploadPrompt="Drag and drop manuals here"
            @upload="uploadManuals"
          />

          <div v-if="loadingManuals" class="loading">Loading manuals...</div>
          <FileViewer
            v-else
            :files="manuals"
            fileType="manuals"
            :entityId="commodity.id"
            entityType="commodities"
            @delete="deleteManual"
            @update="updateManual"
            @download="downloadManual"
          />
        </div>

        <!-- Invoices Section -->
        <div class="info-card full-width">
          <div class="section-header">
            <h2>Invoices</h2>
            <button class="btn btn-sm btn-primary" @click="showInvoiceUploader = !showInvoiceUploader">
              {{ showInvoiceUploader ? 'Cancel' : 'Add Invoices' }}
            </button>
          </div>

          <FileUploader
            v-if="showInvoiceUploader"
            :multiple="true"
            accept=".pdf,.gif,.jpg,.jpeg,.png,.webp,application/pdf,image/gif,image/jpeg,image/png,image/webp"
            uploadPrompt="Drag and drop invoices here"
            @upload="uploadInvoices"
          />

          <div v-if="loadingInvoices" class="loading">Loading invoices...</div>
          <FileViewer
            v-else
            :files="invoices"
            fileType="invoices"
            :entityId="commodity.id"
            entityType="commodities"
            @delete="deleteInvoice"
            @update="updateInvoice"
            @download="downloadInvoice"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import commodityService from '@/services/commodityService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import FileUploader from '@/components/FileUploader.vue'
import FileList from '@/components/FileList.vue'
// import ImageViewer from '@/components/ImageViewer.vue' // Using FileViewer for all file types now
import FileViewer from '@/components/FileViewer.vue'

const router = useRouter()
const route = useRoute()
const commodity = ref<any>(null)
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// File states
const images = ref<any[]>([])
const manuals = ref<any[]>([])
const invoices = ref<any[]>([])

// Loading states for files
const loadingImages = ref<boolean>(false)
const loadingManuals = ref<boolean>(false)
const loadingInvoices = ref<boolean>(false)

// Toggle states for file uploaders
const showImageUploader = ref<boolean>(false)
const showManualUploader = ref<boolean>(false)
const showInvoiceUploader = ref<boolean>(false)

onMounted(async () => {
  const id = route.params.id as string

  try {
    const response = await commodityService.getCommodity(id)
    commodity.value = response.data.data
    loading.value = false

    // Load files after commodity is loaded
    loadFiles()
  } catch (err: any) {
    error.value = 'Failed to load commodity: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const loadFiles = async () => {
  if (!commodity.value) return

  // Load images
  loadingImages.value = true
  try {
    const response = await commodityService.getImages(commodity.value.id)
    images.value = response.data?.data || []
  } catch (err: any) {
    console.error('Failed to load images:', err)
  } finally {
    loadingImages.value = false
  }

  // Load manuals
  loadingManuals.value = true
  try {
    const response = await commodityService.getManuals(commodity.value.id)
    manuals.value = response.data?.data || []
  } catch (err: any) {
    console.error('Failed to load manuals:', err)
  } finally {
    loadingManuals.value = false
  }

  // Load invoices
  loadingInvoices.value = true
  try {
    const response = await commodityService.getInvoices(commodity.value.id)
    invoices.value = response.data?.data || []
  } catch (err: any) {
    console.error('Failed to load invoices:', err)
  } finally {
    loadingInvoices.value = false
  }
}

const uploadImages = async (files: File[]) => {
  if (!commodity.value || files.length === 0) return

  try {
    await commodityService.uploadImages(commodity.value.id, files)
    showImageUploader.value = false
    // Reload images after upload
    loadingImages.value = true
    const response = await commodityService.getImages(commodity.value.id)
    images.value = response.data?.data || []
    loadingImages.value = false
  } catch (err: any) {
    error.value = 'Failed to upload images: ' + (err.message || 'Unknown error')
  }
}

const uploadManuals = async (files: File[]) => {
  if (!commodity.value || files.length === 0) return

  try {
    await commodityService.uploadManuals(commodity.value.id, files)
    showManualUploader.value = false
    // Reload manuals after upload
    loadingManuals.value = true
    const response = await commodityService.getManuals(commodity.value.id)
    manuals.value = response.data?.data || []
    loadingManuals.value = false
  } catch (err: any) {
    error.value = 'Failed to upload manuals: ' + (err.message || 'Unknown error')
  }
}

const uploadInvoices = async (files: File[]) => {
  if (!commodity.value || files.length === 0) return

  try {
    await commodityService.uploadInvoices(commodity.value.id, files)
    showInvoiceUploader.value = false
    // Reload invoices after upload
    loadingInvoices.value = true
    const response = await commodityService.getInvoices(commodity.value.id)
    invoices.value = response.data?.data || []
    loadingInvoices.value = false
  } catch (err: any) {
    error.value = 'Failed to upload invoices: ' + (err.message || 'Unknown error')
  }
}

const deleteImage = async (image: any) => {
  if (!commodity.value) return

  try {
    await commodityService.deleteImage(commodity.value.id, image.id)
    // Remove the deleted image from the list
    images.value = images.value.filter(img => img.id !== image.id)
  } catch (err: any) {
    error.value = 'Failed to delete image: ' + (err.message || 'Unknown error')
  }
}

const deleteManual = async (manual: any) => {
  if (!commodity.value) return

  try {
    await commodityService.deleteManual(commodity.value.id, manual.id)
    // Remove the deleted manual from the list
    manuals.value = manuals.value.filter(m => m.id !== manual.id)
  } catch (err: any) {
    error.value = 'Failed to delete manual: ' + (err.message || 'Unknown error')
  }
}

const deleteInvoice = async (invoice: any) => {
  if (!commodity.value) return

  try {
    await commodityService.deleteInvoice(commodity.value.id, invoice.id)
    // Remove the deleted invoice from the list
    invoices.value = invoices.value.filter(inv => inv.id !== invoice.id)
  } catch (err: any) {
    error.value = 'Failed to delete invoice: ' + (err.message || 'Unknown error')
  }
}

// Download functions
const downloadImage = (image: any) => {
  if (!commodity.value) return

  // Create a link and trigger download
  const link = document.createElement('a')
  const imageUrl = `/api/v1/commodities/${commodity.value.id}/images/${image.id}.${image.ext}`
  link.href = imageUrl
  link.download = image.path + image.ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

const downloadManual = (manual: any) => {
  if (!commodity.value) return

  // Create a link and trigger download
  const link = document.createElement('a')
  const manualUrl = `/api/v1/commodities/${commodity.value.id}/manuals/${manual.id}.${manual.ext}`
  link.href = manualUrl
  link.download = manual.path + manual.ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

const downloadInvoice = (invoice: any) => {
  if (!commodity.value) return

  // Create a link and trigger download
  const link = document.createElement('a')
  const invoiceUrl = `/api/v1/commodities/${commodity.value.id}/invoices/${invoice.id}.${invoice.ext}`
  link.href = invoiceUrl
  link.download = invoice.path + invoice.ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

// File update functions
const updateImage = async (data: any) => {
  if (!commodity.value) return

  try {
    await commodityService.updateImage(commodity.value.id, data.id, { path: data.path })
    // Update the image in the list
    const index = images.value.findIndex(img => img.id === data.id)
    if (index !== -1) {
      images.value[index].path = data.path
    }
  } catch (err: any) {
    error.value = 'Failed to update image: ' + (err.message || 'Unknown error')
  }
}

const updateManual = async (data: any) => {
  if (!commodity.value) return

  try {
    await commodityService.updateManual(commodity.value.id, data.id, { path: data.path })
    // Update the manual in the list
    const index = manuals.value.findIndex(m => m.id === data.id)
    if (index !== -1) {
      manuals.value[index].path = data.path
    }
  } catch (err: any) {
    error.value = 'Failed to update manual: ' + (err.message || 'Unknown error')
  }
}

const updateInvoice = async (data: any) => {
  if (!commodity.value) return

  try {
    await commodityService.updateInvoice(commodity.value.id, data.id, { path: data.path })
    // Update the invoice in the list
    const index = invoices.value.findIndex(inv => inv.id === data.id)
    if (index !== -1) {
      invoices.value[index].path = data.path
    }
  } catch (err: any) {
    error.value = 'Failed to update invoice: ' + (err.message || 'Unknown error')
  }
}

const editCommodity = () => {
  router.push(`/commodities/${commodity.value.id}/edit`)
}

const confirmDelete = () => {
  if (confirm('Are you sure you want to delete this commodity?')) {
    deleteCommodity()
  }
}

const deleteCommodity = async () => {
  try {
    await commodityService.deleteCommodity(commodity.value.id)
    router.push('/commodities')
  } catch (err: any) {
    error.value = 'Failed to delete commodity: ' + (err.message || 'Unknown error')
  }
}

const getTypeName = (typeId: string): string => {
  const type = COMMODITY_TYPES.find(t => t.id === typeId)
  return type ? type.name : typeId
}

const getStatusName = (statusId: string): string => {
  const status = COMMODITY_STATUSES.find(s => s.id === statusId)
  return status ? status.name : statusId
}

const formatDate = (date: string): string => {
  const options: Intl.DateTimeFormatOptions = { year: 'numeric', month: 'long', day: 'numeric' }
  return new Date(date).toLocaleDateString('en-US', options)
}
</script>

<style scoped>
.commodity-detail {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.loading, .error, .not-found {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.error {
  color: #dc3545;
}

.commodity-info {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.5rem;
}

.info-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.info-card h2 {
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.info-row {
  display: flex;
  margin-bottom: 0.75rem;
}

.label {
  font-weight: 500;
  width: 120px;
  color: #555;
}

.status {
  font-weight: 500;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  margin-left: 0.5rem;
}

.status.in_use {
  background-color: #d4edda;
  color: #155724;
}

.status.sold {
  background-color: #cce5ff;
  color: #004085;
}

.status.lost {
  background-color: #fff3cd;
  color: #856404;
}

.status.disposed {
  background-color: #f8d7da;
  color: #721c24;
}

.status.written_off {
  background-color: #e2e3e5;
  color: #383d41;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.tag {
  background-color: #e9ecef;
  color: #333;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  margin-right: 0.5rem;
}

.btn {
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-weight: 500;
  cursor: pointer;
  border: none;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
  text-decoration: none;
  display: inline-block;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-danger {
  background-color: #dc3545;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  margin-top: 0;
  border-radius: 4px;
}

/* New styles for file upload sections */
.full-width {
  grid-column: 1 / -1; /* Make the card span all columns */
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.section-header h2 {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

@media (min-width: 768px) {
  .commodity-info {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
