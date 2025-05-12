<template>
  <div class="inline-form area-form">
    <form @submit.prevent="submitForm">
      <div class="form-row">
        <div class="form-group">
          <label for="name">Area Name</label>
          <input
            type="text"
            id="name"
            v-model="form.name"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.name }"
            placeholder="Enter area name"
          >
          <div v-if="errors.name" class="error-message">{{ errors.name }}</div>
        </div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="cancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Area' }}
        </button>
      </div>

      <div v-if="error" class="form-error">{{ error }}</div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import areaService from '@/services/areaService'

const props = defineProps({
  locationId: {
    type: String,
    required: true
  }
})

const emit = defineEmits(['created', 'cancel'])

const isSubmitting = ref(false)
const error = ref<string | null>(null)

const form = reactive({
  name: '',
  locationId: props.locationId
})

const errors = reactive({
  name: ''
})

const validateForm = () => {
  let isValid = true

  // Reset errors
  errors.name = ''

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    isValid = false
  }

  return isValid
}

const submitForm = async () => {
  if (!validateForm()) return

  isSubmitting.value = true
  error.value = null

  try {
    // Create the payload
    const payload = {
      data: {
        type: 'areas',
        attributes: {
          name: form.name.trim(),
          location_id: props.locationId
        }
      }
    }

    const response = await areaService.createArea(payload)
    
    // Reset form
    form.name = ''
    
    // Emit created event with the new area
    emit('created', response.data.data)
  } catch (err: any) {
    if (err.response && err.response.data && err.response.data.errors) {
      const apiErrors = err.response.data.errors
      apiErrors.forEach((apiError: any) => {
        if (apiError.source && apiError.source.pointer) {
          const field = apiError.source.pointer.split('/').pop()
          if (field === 'name') {
            errors.name = apiError.detail
          }
        }
      })

      if (Object.values(errors).some(e => e)) {
        error.value = 'Please correct the errors above.'
      } else {
        error.value = `Failed to create area: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      error.value = 'Failed to create area: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const cancel = () => {
  // Reset form
  form.name = ''
  
  // Emit cancel event
  emit('cancel')
}
</script>

<style lang="scss" scoped>
.inline-form {
  background: white;
  padding: 1.5rem;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  margin-bottom: 1rem;
  margin-top: 0.5rem;
  margin-left: 2rem; /* Indent to show hierarchy */
}

.form-group {
  margin-bottom: 1rem;
}

label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
}

.form-control {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  font-size: 1rem;
  
  &:focus {
    outline: none;
    border-color: $primary-color;
    box-shadow: 0 0 0 2px rgba($primary-color, 0.2);
  }
  
  &.is-invalid {
    border-color: $danger-color;
  }
}

.error-message {
  color: $danger-color;
  font-size: 0.875rem;
  margin-top: 0.25rem;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 1rem;
}

.form-error {
  color: $danger-color;
  margin-top: 1rem;
  padding: 0.5rem;
  background-color: rgba($danger-color, 0.1);
  border-radius: $default-radius;
}
</style>
