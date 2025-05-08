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
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import commodityService from '@/services/commodityService'

const router = useRouter()
const route = useRoute()
const commodity = ref<any>(null)
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

onMounted(async () => {
  const id = route.params.id as string

  try {
    const response = await commodityService.getCommodity(id)
    commodity.value = response.data.data
    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load commodity: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const editCommodity = () => {
  router.push(`/commodities/${commodity.value.id}/edit`)
}

const confirmDelete = () => {
  if (confirm('Are you sure you want to delete this commodity?')) {
    deleteCommodity
  }
}

const deleteCommodity = async () => {
  try {
    await commodityService.deleteCommodity(commodity.value.id)
    alert('Commodity deleted successfully')
    router.push('/commodities')
  } catch (err: any) {
    error.value = 'Failed to delete commodity: ' + (err.message || 'Unknown error')
  }
}

const getTypeName = (type: string): string => {
  switch (type) {
    case 'type1':
      return 'Type 1'
    case 'type2':
      return 'Type 2'
    // Add more types as needed
    default:
      return 'Unknown Type'
  }
}

const getStatusName = (status: string): string => {
  switch (status) {
    case 'active':
      return 'Active'
    case 'inactive':
      return 'Inactive'
    // Add more statuses as needed
    default:
      return 'Unknown Status'
  }
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

.status.active {
  background-color: #d4edda;
  color: #155724;
}

.status.inactive {
  background-color: #f8d7da;
  color: #721c24;
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

@media (min-width: 768px) {
  .commodity-info {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
