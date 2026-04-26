<template>
  <div class="inline-form location-form rounded-md border bg-card p-6 shadow-sm mb-6">
    <form class="flex flex-col gap-4" @submit.prevent="submitForm">
      <div class="grid gap-4 md:grid-cols-2">
        <div class="flex flex-col gap-2">
          <Label for="name">Name</Label>
          <Input
            id="name"
            v-model="form.name"
            type="text"
            required
            :aria-invalid="!!errors.name"
            placeholder="Enter location name"
          />
          <div v-if="errors.name" class="error-message text-sm text-destructive">{{ errors.name }}</div>
        </div>

        <div class="flex flex-col gap-2">
          <Label for="address">Address</Label>
          <Input
            id="address"
            v-model="form.address"
            type="text"
            required
            :aria-invalid="!!errors.address"
            placeholder="Enter location address"
          />
          <div v-if="errors.address" class="error-message text-sm text-destructive">{{ errors.address }}</div>
        </div>
      </div>

      <FormFooter>
        <Button type="button" variant="outline" @click="cancel">Cancel</Button>
        <Button type="submit" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Location' }}
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
import locationService from '@/services/locationService'

const emit = defineEmits(['created', 'cancel'])

const isSubmitting = ref(false)
const error = ref<string | null>(null)

const form = reactive({
  name: '',
  address: ''
})

const errors = reactive({
  name: '',
  address: ''
})

const validateForm = () => {
  let isValid = true

  // Reset errors
  errors.name = ''
  errors.address = ''

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    isValid = false
  }

  if (!form.address.trim()) {
    errors.address = 'Address is required'
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
        type: 'locations',
        attributes: {
          name: form.name.trim(),
          address: form.address.trim()
        }
      }
    }

    const response = await locationService.createLocation(payload)

    // Reset form
    form.name = ''
    form.address = ''

    // Emit created event with the new location
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
            } else if (field === 'address') {
              errors.address = apiError.detail || ''
            }
          }
        })

        if (Object.values(errors).some(e => e)) {
          error.value = 'Please correct the errors above.'
        } else {
          error.value = `Failed to create location: ${errorResponse.response?.status || 'Unknown'} - ${JSON.stringify(errorResponse.response?.data || {})}`
        }
      } else {
        error.value = 'Failed to create location: Unknown server error'
      }
    } else {
      const errorMessage = err && typeof err === 'object' && 'message' in err ? (err as Error).message : 'Unknown error'
      error.value = 'Failed to create location: ' + errorMessage
    }
  } finally {
    isSubmitting.value = false
  }
}

const cancel = () => {
  // Reset form
  form.name = ''
  form.address = ''

  // Emit cancel event
  emit('cancel')
}
</script>
