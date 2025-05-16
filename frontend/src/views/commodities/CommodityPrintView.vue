<template>
  <div class="commodity-print-view">
    <!-- Toolbar (hidden when printing) -->
    <div class="print-toolbar">
      <div class="toolbar-section">
        <label>
          <input v-model="colorMode" type="radio" value="color" name="colorMode" />
          Color
        </label>
        <label>
          <input v-model="colorMode" type="radio" value="bw" name="colorMode" />
          Black & White
        </label>
      </div>

      <div class="toolbar-section" :class="{ disabled: colorMode === 'bw' }">
        <label>
          <input
            v-model="imageColorMode"
            type="radio"
            value="color"
            name="imageColorMode"
            :disabled="colorMode === 'bw'"
          />
          Images in Color
        </label>
        <label>
          <input
            v-model="imageColorMode"
            type="radio"
            value="bw"
            name="imageColorMode"
            :disabled="colorMode === 'bw'"
          />
          Images in B&W
        </label>
      </div>

      <div class="toolbar-section">
        <label>Image Layout:</label>
        <select v-model="imageLayout">
          <option value="1">1 per page</option>
          <option value="4">4 per page</option>
        </select>
      </div>

      <div class="toolbar-section">
        <button class="btn btn-primary" @click="print">
          <font-awesome-icon icon="print" /> Print
        </button>
        <button class="btn btn-secondary" @click="goBack">
          <font-awesome-icon icon="arrow-left" /> Back
        </button>
      </div>
    </div>

    <!-- Commodity Details -->
    <div class="print-content" :class="{ 'bw': colorMode === 'bw' }">
      <div v-if="loading" class="loading">Loading...</div>
      <div v-else-if="error" class="error">{{ error }}</div>
      <div v-else-if="!commodity" class="not-found">Commodity not found</div>
      <div v-else class="commodity-details">
        <div class="print-header">
          <h1>{{ commodity.attributes.name }}</h1>
          <div class="print-timestamp">Printed on: {{ currentDateTime }}</div>
        </div>

        <div class="info-section location-area-info">
          <div v-if="location" class="info-row">
            <span class="label">Location:</span>
            <span>{{ location.attributes.name }}</span>
          </div>
          <div v-if="location && location.attributes.address" class="info-row">
            <span class="label">Address:</span>
            <span>{{ location.attributes.address }}</span>
          </div>
          <div v-if="area" class="info-row">
            <span class="label">Area:</span>
            <span>{{ area.attributes.name }}</span>
          </div>
        </div>

        <div class="info-section">
          <h2>Basic Information</h2>
          <div class="info-grid">
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
        </div>

        <div class="info-section">
          <h2>Price Information</h2>
          <div class="info-grid">
            <div class="info-row">
              <span class="label">Original Price:</span>
              <span>{{ formatPrice(parseFloat(commodity.attributes.original_price) || 0, commodity.attributes.original_price_currency) }}</span>
            </div>
            <div v-if="commodity.attributes.converted_original_price !== '0' && commodity.attributes.converted_original_price !== 0" class="info-row">
              <span class="label">Converted Original Price:</span>
              <span>{{ formatPrice(parseFloat(commodity.attributes.converted_original_price) || 0, mainCurrency) }}</span>
            </div>
            <div v-if="!(commodity.attributes.original_price_currency === mainCurrency && parseFloat(commodity.attributes.original_price) > 0)" class="info-row">
              <span class="label">Current Price:</span>
              <span>{{ formatPrice(parseFloat(commodity.attributes.current_price) || 0, mainCurrency) }}</span>
            </div>
          </div>
        </div>

        <div v-if="commodity.attributes.serial_number || (commodity.attributes.extra_serial_numbers && commodity.attributes.extra_serial_numbers.length > 0) || (commodity.attributes.part_numbers && commodity.attributes.part_numbers.length > 0)" class="info-section">
          <h2>Serial Numbers and Part Numbers</h2>
          <div class="info-grid">
            <div v-if="commodity.attributes.serial_number" class="info-row">
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
        </div>

        <div v-if="commodity.attributes.tags && commodity.attributes.tags.length > 0" class="info-section">
          <h2>Tags</h2>
          <div class="tags">
            <span v-for="(tag, index) in commodity.attributes.tags" :key="index" class="tag">
              {{ tag }}
            </span>
          </div>
        </div>

        <div v-if="commodity.attributes.urls && commodity.attributes.urls.length > 0" class="info-section">
          <h2>URLs</h2>
          <ul>
            <li v-for="(url, index) in commodity.attributes.urls" :key="index">
              <a :href="url" target="_blank">{{ url }}</a>
            </li>
          </ul>
        </div>

        <!-- Images Section -->
        <div v-if="images.length > 0" class="info-section">
          <div class="images-container" :class="[`layout-${imageLayout}`]">
            <div v-for="image in images" :key="image.id" class="image-item">
              <img
                :src="getImageUrl(image)"
                :alt="getImageName(image)"
                :class="{
                  'bw': colorMode === 'bw' || (colorMode === 'color' && imageColorMode === 'bw')
                }"
              />
              <div class="image-caption">{{ getImageName(image) }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import commodityService from '@/services/commodityService'
import { useSettingsStore } from '@/stores/settingsStore'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import { formatPrice } from '@/utils/priceUtils'

const route = useRoute()

const settingsStore = useSettingsStore()
const commodity = ref<any>(null)
const location = ref<any>(null)
const area = ref<any>(null)
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const images = ref<any[]>([])
const loadingImages = ref<boolean>(false)

// Use the main currency from the store
const mainCurrency = computed(() => settingsStore.mainCurrency)

// Current date and time for the timestamp
const currentDateTime = ref(new Date().toLocaleString())

// Print options
const colorMode = ref<'color' | 'bw'>('color')
const imageColorMode = ref<'color' | 'bw'>('color')
const imageLayout = ref<'1' | '4'>('1')

// Watch for changes in colorMode
watch(colorMode, (newValue) => {
  if (newValue === 'bw') {
    imageColorMode.value = 'bw'
  }
})

onMounted(async () => {
  const id = route.params.id as string

  try {
    // Fetch main currency from the store
    await settingsStore.fetchMainCurrency()

    // Load commodity data
    const response = await commodityService.getCommodity(id)
    commodity.value = response.data.data

    // Load area if available
    if (commodity.value.attributes.area_id) {
      try {
        const areaResponse = await fetch(`/api/v1/areas/${commodity.value.attributes.area_id}`, {
          headers: { 'Accept': 'application/vnd.api+json' }
        })
        if (areaResponse.ok) {
          const areaData = await areaResponse.json()
          area.value = areaData.data

          // Load location if area has a location_id
          if (area.value.attributes.location_id) {
            try {
              const locationResponse = await fetch(`/api/v1/locations/${area.value.attributes.location_id}`, {
                headers: { 'Accept': 'application/vnd.api+json' }
              })
              if (locationResponse.ok) {
                const locationData = await locationResponse.json()
                location.value = locationData.data
              }
            } catch (locErr) {
              console.error('Failed to load location:', locErr)
            }
          }
        }
      } catch (areaErr) {
        console.error('Failed to load area:', areaErr)
      }
    }

    loading.value = false

    // Load images
    await loadImages()
  } catch (err: any) {
    error.value = 'Failed to load commodity: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const loadImages = async () => {
  if (!commodity.value) return

  loadingImages.value = true
  try {
    const response = await commodityService.getImages(commodity.value.id)
    images.value = response.data?.data || []
  } catch (err: any) {
    console.error('Failed to load images:', err)
  } finally {
    loadingImages.value = false
  }
}

const getTypeName = (type: string) => {
  return COMMODITY_TYPES.find(t => t.value === type)?.label || type
}

const getStatusName = (status: string) => {
  return COMMODITY_STATUSES.find(s => s.value === status)?.label || status
}

const formatDate = (dateString: string) => {
  if (!dateString) return 'N/A'

  const date = new Date(dateString)
  return date.toLocaleDateString()
}

const getImageUrl = (image: any) => {
  return `/api/v1/commodities/${commodity.value.id}/images/${image.id}${image.ext}`
}

const getImageName = (image: any) => {
  if (!image.path) return `Image ${image.id}`
  return image.path + image.ext
}

const print = () => {
  window.print()
}

const goBack = () => {
  window.close()
}
</script>

<style lang="scss" scoped>
@import '../../assets/main.scss';

.commodity-print-view {
  max-width: 100%;
  margin: 0;
  padding: 20px;
}

.print-toolbar {
  background-color: $light-bg-color;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  padding: 1rem;
  margin-bottom: 2rem;
  display: flex;
  flex-wrap: wrap;
  gap: 1.5rem;
  align-items: center;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  box-shadow: $box-shadow;
}

.toolbar-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;

  &:last-child {
    flex-direction: row;
  }

  &.disabled {
    opacity: 0.5;
    pointer-events: none;
  }

  label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
  }

  select {
    padding: 0.5rem;
    border-radius: $default-radius;
    border: 1px solid $border-color;
  }
}

.print-content {
  background-color: white;
  border-radius: $default-radius;
  padding: 2rem;
  box-shadow: $box-shadow;
  margin-top: 80px; /* Space for the fixed toolbar */
}

.print-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #eee;
}

.print-timestamp {
  font-size: 0.9rem;
  color: $secondary-color;
  text-align: right;
}

.location-area-info {
  background-color: $light-bg-color;
  border-left: 4px solid $primary-color;
  padding: 1rem;
  margin-bottom: 2rem;
}

.print-content.bw {
  filter: grayscale(100%);
}

.commodity-details h1 {
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #eee;
}

.info-section {
  margin-bottom: 2rem;
}

.info-section h2 {
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.info-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.75rem;
}

.info-row {
  display: flex;
  align-items: baseline;
}

.label {
  font-weight: 500;
  width: 180px;
  color: #555;
}

.status {
  font-weight: 500;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
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
}

.images-container {
  display: grid;
  gap: 2rem;
  margin-top: 1rem;
}

.images-container.layout-1 {
  grid-template-columns: 1fr;
}

.images-container.layout-1 .image-item {
  height: 100vh;
  margin-bottom: 2rem;
}

.images-container.layout-4 {
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
}

.images-container.layout-4 .image-item {
  height: 40vh;
}

.image-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  page-break-inside: avoid;
  height: 100%;
}

.image-item img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border: 1px solid #dee2e6;
  border-radius: 4px;
  max-height: 80vh;
}

.image-item img.bw {
  filter: grayscale(100%);
}

.image-caption {
  margin-top: 0.5rem;
  text-align: center;
  font-size: 0.9rem;
  color: #555;
}

/* Print media styles */
@media print {
  .print-toolbar {
    display: none !important;
  }

  .commodity-print-view {
    padding: 0;
  }

  .print-content {
    box-shadow: none;
    padding: 0;
  }

  .info-section {
    page-break-inside: avoid;
  }

  /* Image layout handling for print */
  .images-container.layout-1 .image-item {
    page-break-before: always;
    height: 100vh;
    margin-bottom: 2rem;
  }

  .images-container.layout-4 .image-item {
    height: 35vh;
    page-break-inside: avoid;
  }

  .image-item img {
    max-height: 90%;
    max-width: 90%;
  }

  /* Force background colors to print */
  .status {
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }

  /* Hide navigation elements */
  header, footer, nav {
    display: none !important;
  }

  /* Ensure white background */
  body {
    background-color: white !important;
  }
}
</style>
