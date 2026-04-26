<template>
  <div class="inline-form area-form rounded-md border bg-card p-6 shadow-sm ml-8 mt-2 mb-4">
    <form class="flex flex-col gap-4" @submit.prevent="submitForm">
      <div class="flex flex-col gap-2">
        <Label for="name">Area Name</Label>
        <Input
          id="name"
          v-model="form.name"
          type="text"
          required
          :aria-invalid="!!errors.name"
          placeholder="Enter area name"
        />
        <div v-if="errors.name" class="error-message text-sm text-destructive">{{ errors.name }}</div>
      </div>

      <FormFooter>
        <Button type="button" variant="outline" @click="cancel">Cancel</Button>
        <Button type="submit" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Area' }}
        </Button>
      </FormFooter>

      <Banner v-if="error" variant="error">{{ error }}</Banner>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import Banner from '@design/patterns/Banner.vue'
import FormFooter from '@design/patterns/FormFooter.vue'
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
  } catch (err: unknown) {
    if (err && typeof err === 'object' && 'response' in err) {
      const errorResponse = err as { response?: { data?: { errors?: Array<{ source?: { pointer?: string }, detail?: string }> } } }
      if (errorResponse.response?.data?.errors) {
        const apiErrors = errorResponse.response.data.errors
        apiErrors.forEach((apiError) => {
          if (apiError.source && apiError.source.pointer) {
            const field = apiError.source.pointer.split('/').pop()
            if (field === 'name') {
              errors.name = apiError.detail || ''
            }
          }
        })

        if (Object.values(errors).some(e => e)) {
          error.value = 'Please correct the errors above.'
        } else {
          error.value = `Failed to create area: ${errorResponse.response?.status || 'Unknown'} - ${JSON.stringify(errorResponse.response?.data || {})}`
        }
      } else {
        error.value = 'Failed to create area: Unknown server error'
      }
    } else {
      const errorMessage = err && typeof err === 'object' && 'message' in err ? (err as Error).message : 'Unknown error'
      error.value = 'Failed to create area: ' + errorMessage
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
