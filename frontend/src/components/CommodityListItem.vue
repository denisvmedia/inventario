<template>
  <div class="commodity-card" :class="{
            'highlighted': commodity.id === highlightCommodityId,
            'draft': commodity.attributes.draft,
            'sold': !commodity.attributes.draft && commodity.attributes.status === 'sold',
            'lost': !commodity.attributes.draft && commodity.attributes.status === 'lost',
            'disposed': !commodity.attributes.draft && commodity.attributes.status === 'disposed',
            'written-off': !commodity.attributes.draft && commodity.attributes.status === 'written_off'
          }" @click="viewCommodity(commodity.id)">
    <div class="commodity-content">
      <h3>{{ commodity.attributes.name }}</h3>
      <div v-if="showLocation && commodity.attributes.area_id" class="commodity-location">
        <span class="location-info">
          <font-awesome-icon icon="map-marker-alt" />
          {{ getLocationName(commodity.attributes.area_id) }} / {{ getAreaName(commodity.attributes.area_id) }}
        </span>
      </div>
      <div class="commodity-meta">
        <span class="type">
          <font-awesome-icon :icon="getTypeIcon(commodity.attributes.type)" />
          {{ getTypeName(commodity.attributes.type) }}
        </span>
        <span v-if="(commodity.attributes.count || 1) > 1" class="count">×{{ commodity.attributes.count }}</span>
      </div>
      <div v-if="commodity.attributes.purchase_date" class="commodity-purchase-date">
        <font-awesome-icon icon="calendar" />
        {{ formatPurchaseDate(commodity.attributes.purchase_date) }}
      </div>
      <div class="commodity-price">
        <span class="price">{{ formatPrice(getDisplayPrice(commodity)) }}</span>
        <span v-if="(commodity.attributes.count || 1) > 1" class="price-per-unit">
          {{ formatPrice(calculatePricePerUnit(commodity)) }} per unit
        </span>
      </div>
      <div v-if="commodity.attributes.status" class="commodity-status" :class="{ 'with-draft': commodity.attributes.draft }">
        <span class="status" :class="commodity.attributes.status">{{ getStatusName(commodity.attributes.status) }}</span>
      </div>
    </div>
    <div class="commodity-actions">
      <button class="btn btn-secondary btn-sm" title="Edit" @click.stop="editCommodity(commodity.id)">
        <font-awesome-icon icon="edit" />
      </button>
      <button class="btn btn-danger btn-sm" title="Delete" @click.stop="confirmDeleteCommodity(commodity.id)">
        <font-awesome-icon icon="trash" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import {calculatePricePerUnit, formatPrice, getDisplayPrice} from "@/services/currencyService.ts";
import {COMMODITY_STATUSES} from "@/constants/commodityStatuses.ts";
import {COMMODITY_TYPES} from "@/constants/commodityTypes.ts";

const props = defineProps({
  commodity: {
    type: Object,
    required: true
  },
  highlightCommodityId: {
    type: String,
    default: ''
  },
  showLocation: {
    type: Boolean,
    default: false
  },
  areaMap: {
    type: Object,
    default: () => ({})
  },
  locationMap: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['view-commodity', 'edit-commodity', 'confirm-delete-commodity'])

const viewCommodity = (id: string) => {
  emit('view-commodity', id)
}

const editCommodity = (id: string) => {
  emit('edit-commodity', id)
}

const confirmDeleteCommodity = (id: string) => {
  emit('confirm-delete-commodity', id)
}

const getTypeIcon = (typeId: string) => {
  switch(typeId) {
    case 'white_goods': return 'blender'
    case 'electronics': return 'laptop'
    case 'equipment': return 'tools'
    case 'furniture': return 'couch'
    case 'clothes': return 'tshirt'
    case 'other': return 'box'
    default: return 'box'
  }
}

const getTypeName = (typeId: string) => {
  const type = COMMODITY_TYPES.find(t => t.id === typeId)
  return type ? type.name : typeId
}

const getStatusName = (statusId: string) => {
  const status = COMMODITY_STATUSES.find(s => s.id === statusId)
  return status ? status.name : statusId
}

const getAreaName = (areaId: string) => {
  return props.areaMap[areaId]?.name || 'Unknown Area'
}

const getLocationName = (areaId: string) => {
  const locationId = props.areaMap[areaId]?.locationId
  return props.locationMap[locationId]?.name || 'Unknown Location'
}

const formatPurchaseDate = (date: string): string => {
  const options: Intl.DateTimeFormatOptions = { year: 'numeric', month: 'short', day: 'numeric' }
  return new Date(date).toLocaleDateString('en-US', options)
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.commodity-location {
  margin-top: 0.5rem;
  font-size: 0.85rem;
  color: $text-color;
}

.commodity-meta {
  display: flex;
  justify-content: space-between;
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: $text-color;
}

.commodity-purchase-date {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-top: 0.5rem;
  font-size: 0.85rem;
  color: $text-color;
}

.commodity-price {
  margin-top: 1rem;
  font-weight: bold;
  font-size: 1.1rem;
  display: flex;
  flex-direction: column;
}

.price-per-unit {
  font-size: 0.8rem;
  font-weight: normal;
  font-style: italic;
  color: $text-color;
  margin-top: 0.25rem;
}

.status {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: $default-radius;
  font-size: 0.8rem;
  font-weight: 500;

  &.in_use {
    background-color: #d4edda;
    color: #155724;
  }

  &.sold {
    background-color: #cce5ff;
    color: #004085;
  }

  &.lost {
    background-color: #fff3cd;
    color: #856404;
  }

  &.disposed {
    background-color: #f8d7da;
    color: #721c24;
  }

  &.written_off {
    background-color: #e2e3e5;
    color: #383d41;
  }
}

.commodity-card {
  background: white;
  border-radius: $default-radius;
  padding: 1.5rem;
  box-shadow: $box-shadow;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;

  &:hover {
    transform: translateY(-5px);
    box-shadow: 0 5px 15px rgb(0 0 0 / 10%);
  }

  &.highlighted {
    border-left: 4px solid $primary-color;
    box-shadow: 0 2px 10px rgba($primary-color, 0.3);
    background-color: #f9fff9;
  }

  &.draft {
    background: repeating-linear-gradient(45deg, #fff, #fff 5px, #eeeeee4d 5px, #eeeeee4d 7px);
    position: relative;
    filter: grayscale(0.8);

    h3, .commodity-location, .commodity-meta, .commodity-price, .price-per-unit {
      color: $text-secondary-color;
    }

    .status {
      background-color: #e2e3e5 !important;
      color: #383d41 !important;
    }
  }

  &.sold {
    position: relative;
    filter: grayscale(0.8);

    &::before {
      content: 'SOLD';
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%) rotate(-45deg);
      font-size: 2.5rem;
      font-weight: bold;
      color: rgb(204 229 255 / 80%);
      border: 3px solid rgb(0 64 133 / 50%);
      padding: 0.5rem 1rem;
      border-radius: $default-radius;
      z-index: 1;
      pointer-events: none;
    }
  }

  &.lost {
    position: relative;
    filter: saturate(0.7);

    &::before {
      content: '';
      position: absolute;
      inset: 0;
      background-color: rgb(255 243 205 / 30%);
      z-index: 1;
      pointer-events: none;
    }

    &::after {
      content: '⚠️';
      position: absolute;
      bottom: 1rem;
      right: 1rem;
      font-size: 1.5rem;
      z-index: 2;
      pointer-events: none;
    }
  }

  &.disposed {
    position: relative;

    &::before {
      content: '';
      position: absolute;
      inset: 0;
      background-color: rgb(248 215 218 / 30%);
      background-image: linear-gradient(45deg, transparent, transparent 48%, rgb(114 28 36 / 20%) 49%, rgb(114 28 36 / 20%) 51%, transparent 52%, transparent);
      background-size: 20px 20px;
      z-index: 1;
      pointer-events: none;
    }

    &::after {
      content: '🗑️';
      position: absolute;
      bottom: 1rem;
      right: 1rem;
      font-size: 1.5rem;
      z-index: 2;
      pointer-events: none;
    }
  }

  &.written-off {
    position: relative;
    filter: contrast(0.95);

    &::before {
      content: '';
      position: absolute;
      inset: 0;
      background-color: rgb(226 227 229 / 3.75%);
      background-image:
        linear-gradient(45deg, transparent, transparent 45%, rgb(56 61 65 / 3.75%) 46%, rgb(56 61 65 / 3.75%) 54%, transparent 55%, transparent),
        linear-gradient(135deg, transparent, transparent 45%, rgb(56 61 65 / 3.75%) 46%, rgb(56 61 65 / 3.75%) 54%, transparent 55%, transparent);
      background-size: 30px 30px;
      z-index: 1;
      pointer-events: none;
    }
  }
}

.commodity-content {
  flex: 1;
  cursor: pointer;
}

.commodity-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.location-info {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.type {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.commodity-status {
  margin-top: 0.5rem;

  &.with-draft {
    display: flex;
    justify-content: space-between;
    align-items: center;

    &::after {
      content: 'Draft';
      font-size: 0.8rem;
      font-weight: 500;
      color: $text-secondary-color;
      font-style: italic;
      transform: rotate(-45deg);
      position: absolute;
      bottom: 0.5rem;
      right: 0.5rem;
    }
  }
}
</style>
