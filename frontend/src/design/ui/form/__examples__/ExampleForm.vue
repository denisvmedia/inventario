<script setup lang="ts">
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'

import { Button } from '@design/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@design/ui/form'
import { Input } from '@design/ui/input'

import { exampleFormSchema, type ExampleFormInput } from './ExampleForm.schema'

/**
 * Minimal reference form that mirrors the skeleton in
 * devdocs/frontend/forms.md. Used by form.spec.ts to verify the
 * vee-validate + zod + shadcn-vue `<Form>` pipeline is wired
 * correctly end-to-end. Not imported by any production view.
 */
const emit = defineEmits<{
  (_e: 'submitted', _values: ExampleFormInput): void
}>()

const { handleSubmit, isSubmitting, values, errors } = useForm({
  validationSchema: toTypedSchema(exampleFormSchema),
})

const onSubmit = handleSubmit((submittedValues) => {
  emit('submitted', submittedValues)
})

defineExpose({ values, errors, onSubmit })
</script>

<template>
  <form class="space-y-6" data-testid="example-form" @submit="onSubmit">
    <FormField v-slot="{ componentField }" name="name">
      <FormItem>
        <FormLabel required>Name</FormLabel>
        <FormControl>
          <Input v-bind="componentField" placeholder="Enter the full name" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-slot="{ componentField }" name="email">
      <FormItem>
        <FormLabel required>Email</FormLabel>
        <FormControl>
          <Input v-bind="componentField" type="email" placeholder="you@example.com" />
        </FormControl>
        <FormDescription>Used for recovery only.</FormDescription>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-slot="{ componentField }" name="count">
      <FormItem>
        <FormLabel required>Count</FormLabel>
        <FormControl>
          <Input v-bind="componentField" type="number" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline">Cancel</Button>
      <Button type="submit" :disabled="isSubmitting" data-testid="example-form-submit">
        Submit
      </Button>
    </div>
  </form>
</template>
