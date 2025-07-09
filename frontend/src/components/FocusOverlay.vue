<template>
  <Teleport to="body">
    <!-- Simple overlay that makes the existing button appear above it -->
    <div v-if="show" class="focus-overlay" @click="handleOverlayClick">
      <!-- Semi-transparent backdrop -->
      <div class="overlay-backdrop"></div>

      <!-- Hint message positioned near the target element -->
      <div
        v-if="targetRect"
        class="hint-message"
        :style="hintStyle"
      >
        <div class="hint-content">
          <FontAwesomeIcon icon="info-circle" class="hint-icon" />
          <span>{{ message }}</span>
        </div>
        <div class="hint-arrow" :style="arrowStyle"></div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onUnmounted } from 'vue'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'

interface Props {
  show: boolean
  targetElement?: HTMLElement | null
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  message: 'Don\'t forget to upload your files!'
})

const emit = defineEmits(['close'])

const targetRect = ref<DOMRect | null>(null)

const updateTargetRect = () => {
  if (props.targetElement && props.show) {
    targetRect.value = props.targetElement.getBoundingClientRect()
  } else {
    targetRect.value = null
  }
}

const scrollToTarget = () => {
  if (props.targetElement) {
    const rect = props.targetElement.getBoundingClientRect()
    const isVisible = rect.top >= 0 && rect.bottom <= window.innerHeight
    
    if (!isVisible) {
      props.targetElement.scrollIntoView({
        behavior: 'smooth',
        block: 'center'
      })
      
      // Update rect after scrolling
      setTimeout(() => {
        updateTargetRect()
      }, 300)
    }
  }
}

// Watch for changes in show prop and target element
watch([() => props.show, () => props.targetElement], async () => {
  if (props.show && props.targetElement) {
    await nextTick()
    scrollToTarget()
    updateTargetRect()

    // Set high z-index on target element to make it appear above overlay
    props.targetElement.style.position = 'relative'
    props.targetElement.style.zIndex = '10000'

    // Update rect on window resize
    window.addEventListener('resize', updateTargetRect)
    window.addEventListener('scroll', updateTargetRect)
  } else {
    // Reset z-index when overlay is hidden
    if (props.targetElement) {
      props.targetElement.style.zIndex = ''
      props.targetElement.style.position = ''
    }
    window.removeEventListener('resize', updateTargetRect)
    window.removeEventListener('scroll', updateTargetRect)
  }
}, { immediate: true })



// Computed styles for the hint message
const hintStyle = computed(() => {
  if (!targetRect.value) return {}

  const hintWidth = 280
  const hintHeight = 60
  const arrowSize = 12
  const spacing = 20

  // Position hint above the target element
  let left = targetRect.value.left + (targetRect.value.width / 2) - (hintWidth / 2)
  let top = targetRect.value.top - hintHeight - arrowSize - spacing

  // Ensure hint stays within viewport
  if (left < 20) left = 20
  if (left + hintWidth > window.innerWidth - 20) left = window.innerWidth - hintWidth - 20
  if (top < 20) {
    // If no space above, position below
    top = targetRect.value.bottom + arrowSize + spacing
  }

  return {
    left: `${left}px`,
    top: `${top}px`,
    width: `${hintWidth}px`,
  }
})

// Computed styles for the arrow
const arrowStyle = computed(() => {
  if (!targetRect.value) return {}
  
  const hintLeft = parseInt(hintStyle.value.left as string)
  const hintTop = parseInt(hintStyle.value.top as string)
  const targetCenterX = targetRect.value.left + (targetRect.value.width / 2)
  
  // Calculate arrow position relative to hint
  const arrowLeft = targetCenterX - hintLeft - 6 // 6 is half arrow width
  
  // Determine if arrow should point up or down
  const isAboveTarget = hintTop < targetRect.value.top
  
  return {
    left: `${arrowLeft}px`,
    [isAboveTarget ? 'bottom' : 'top']: '-12px',
    borderTopColor: isAboveTarget ? 'transparent' : '#333',
    borderBottomColor: isAboveTarget ? '#333' : 'transparent',
  }
})

const handleOverlayClick = (event: MouseEvent) => {
  // Close overlay when clicking on backdrop (not on the target element)
  const target = event.target as HTMLElement
  if (target.classList.contains('overlay-backdrop') || target.classList.contains('focus-overlay')) {
    emit('close')
  }
}

// Clean up event listeners on unmount
onUnmounted(() => {
  window.removeEventListener('resize', updateTargetRect)
  window.removeEventListener('scroll', updateTargetRect)
})
</script>

<style lang="scss" scoped>
@use '@/assets/variables' as *;
@use 'sass:color';

.focus-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 9999;
  pointer-events: auto;
}

.overlay-backdrop {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  /* stylelint-disable-next-line color-function-notation, color-function-alias-notation, alpha-value-notation */
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
}

.hint-message {
  position: absolute;
  background: #333;
  color: white;
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 14px;
  /* stylelint-disable-next-line color-function-notation, color-function-alias-notation, alpha-value-notation */
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  animation: fade-in-up 0.3s ease-out;
  pointer-events: none;
  z-index: 9999;
}

.hint-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.hint-icon {
  color: $primary-color;
  flex-shrink: 0;
}

.hint-arrow {
  position: absolute;
  width: 0;
  height: 0;
  border-left: 6px solid transparent;
  border-right: 6px solid transparent;
  border-top: 12px solid #333;
  border-bottom: 12px solid transparent;
}

@keyframes fade-in-up {
  from {
    opacity: 0;
    transform: translateY(10px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
