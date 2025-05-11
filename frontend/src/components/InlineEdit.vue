<template>
  <div class="inline-edit">
    <div v-if="!editing" class="display-value" @click="startEditing">
      <slot name="display">{{ modelValue }}</slot>
      <font-awesome-icon icon="pencil-alt" class="edit-icon" />
    </div>
    <div v-else class="edit-form">
      <input
        v-if="type === 'text'"
        ref="inputRef"
        v-model="editValue"
        :placeholder="placeholder"
        @keyup.enter="save"
        @keyup.esc="cancel"
        class="edit-input"
      />
      <textarea
        v-else-if="type === 'textarea'"
        ref="inputRef"
        v-model="editValue"
        :placeholder="placeholder"
        @keyup.enter="save"
        @keyup.esc="cancel"
        class="edit-textarea"
        :rows="rows"
      ></textarea>
      <div class="edit-actions">
        <button class="btn btn-sm btn-success" @click="save" title="Save">
          <font-awesome-icon icon="check" />
        </button>
        <button class="btn btn-sm btn-secondary" @click="cancel" title="Cancel">
          <font-awesome-icon icon="times" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'

const props = defineProps({
  modelValue: {
    type: [String, Number],
    required: true
  },
  type: {
    type: String,
    default: 'text',
    validator: (value: string) => ['text', 'textarea'].includes(value)
  },
  placeholder: {
    type: String,
    default: 'Enter value'
  },
  rows: {
    type: Number,
    default: 3
  }
})

const emit = defineEmits(['update:modelValue', 'save', 'cancel'])

const editing = ref(false)
const editValue = ref('')
const inputRef = ref<HTMLInputElement | HTMLTextAreaElement | null>(null)

const startEditing = () => {
  editValue.value = String(props.modelValue)
  editing.value = true

  // Focus the input after the DOM updates
  nextTick(() => {
    if (inputRef.value) {
      inputRef.value.focus()
    }
  })
}

const save = () => {
  if (editValue.value.trim() === '') {
    return
  }

  emit('update:modelValue', editValue.value)
  emit('save', editValue.value)
  editing.value = false
}

const cancel = () => {
  editing.value = false
  emit('cancel')
}
</script>

<style scoped>
.inline-edit {
  position: relative;
}

.display-value {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.25rem;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.display-value:hover {
  background-color: rgba(0, 0, 0, 0.05);
}

.edit-icon {
  opacity: 0;
  color: #6c757d;
  font-size: 0.9rem;
  transition: opacity 0.2s;
}

.display-value:hover .edit-icon {
  opacity: 1;
}

.edit-form {
  display: flex;
  gap: 0.5rem;
  align-items: flex-start;
}

.edit-input, .edit-textarea {
  flex: 1;
  padding: 0.375rem 0.75rem;
  border: 1px solid #ced4da;
  border-radius: 0.25rem;
  font-size: 1rem;
}

.edit-textarea {
  resize: vertical;
}

.edit-actions {
  display: flex;
  gap: 0.25rem;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.375rem 0.75rem;
  border: none;
  border-radius: 0.25rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.btn-success {
  background-color: #28a745;
  color: white;
}

.btn-success:hover {
  background-color: #218838;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background-color: #5a6268;
}
</style>
