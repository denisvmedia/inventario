<template>
  <Teleport to="body">
    <div v-if="show" class="focus-overlay" @click="handleOverlayClick">
      <!-- Multi-part backdrop that creates a cutout effect -->
      <template v-if="targetRect">
        <!-- Top backdrop -->
        <div class="overlay-backdrop-part" :style="topBackdropStyle"></div>
        <!-- Bottom backdrop -->
        <div class="overlay-backdrop-part" :style="bottomBackdropStyle"></div>
        <!-- Left backdrop -->
        <div class="overlay-backdrop-part" :style="leftBackdropStyle"></div>
        <!-- Right backdrop -->
        <div class="overlay-backdrop-part" :style="rightBackdropStyle"></div>
      </template>
      <div v-else class="overlay-backdrop"></div>

      <!-- Highlight border for the target element -->
      <div
        v-if="targetRect"
        class="highlight-border"
        :style="cutoutStyle"
      ></div>

      <!-- Hint message -->
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
  allowClickThrough?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  message: 'Don\'t forget to upload your files!',
  allowClickThrough: true
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
    
    // Update rect on window resize
    window.addEventListener('resize', updateTargetRect)
    window.addEventListener('scroll', updateTargetRect)
  } else {
    window.removeEventListener('resize', updateTargetRect)
    window.removeEventListener('scroll', updateTargetRect)
  }
}, { immediate: true })

// Computed styles for the four backdrop parts that create a cutout
const topBackdropStyle = computed(() => {
  if (!targetRect.value) return {}
  const padding = 8
  const cutoutTop = targetRect.value.top - padding

  return {
    top: '0px',
    left: '0px',
    width: '100%',
    height: `${Math.max(0, cutoutTop)}px`
  }
})

const bottomBackdropStyle = computed(() => {
  if (!targetRect.value) return {}
  const padding = 8
  const cutoutBottom = targetRect.value.bottom + padding

  return {
    top: `${cutoutBottom}px`,
    left: '0px',
    width: '100%',
    height: `calc(100% - ${cutoutBottom}px)`
  }
})

const leftBackdropStyle = computed(() => {
  if (!targetRect.value) return {}
  const padding = 8
  const cutoutTop = targetRect.value.top - padding
  const cutoutLeft = targetRect.value.left - padding
  const cutoutHeight = targetRect.value.height + padding * 2

  return {
    top: `${Math.max(0, cutoutTop)}px`,
    left: '0px',
    width: `${Math.max(0, cutoutLeft)}px`,
    height: `${cutoutHeight}px`
  }
})

const rightBackdropStyle = computed(() => {
  if (!targetRect.value) return {}
  const padding = 8
  const cutoutTop = targetRect.value.top - padding
  const cutoutRight = targetRect.value.right + padding
  const cutoutHeight = targetRect.value.height + padding * 2

  return {
    top: `${Math.max(0, cutoutTop)}px`,
    left: `${cutoutRight}px`,
    width: `calc(100% - ${cutoutRight}px)`,
    height: `${cutoutHeight}px`
  }
})

// Computed styles for the highlight border
const cutoutStyle = computed(() => {
  if (!targetRect.value) return {}

  const padding = 8 // Extra padding around the target element

  return {
    left: `${targetRect.value.left - padding}px`,
    top: `${targetRect.value.top - padding}px`,
    width: `${targetRect.value.width + padding * 2}px`,
    height: `${targetRect.value.height + padding * 2}px`,
  }
})

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
  if (props.allowClickThrough && targetRect.value) {
    const rect = targetRect.value
    const clickX = event.clientX
    const clickY = event.clientY
    
    // Check if click is within the target element bounds
    if (clickX >= rect.left && clickX <= rect.right && 
        clickY >= rect.top && clickY <= rect.bottom) {
      // Forward the click to the target element
      props.targetElement?.click()
      return
    }
  }
  
  // Close overlay on backdrop click
  emit('close')
}

// Clean up event listeners on unmount
onUnmounted(() => {
  window.removeEventListener('resize', updateTargetRect)
  window.removeEventListener('scroll', updateTargetRect)
})
</script>

<style lang="scss" scoped>
@use '@/assets/variables' as *;

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
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
}

.overlay-backdrop-part {
  position: absolute;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
}

.highlight-border {
  position: absolute;
  background: transparent;
  border: 3px solid $primary-color;
  border-radius: 8px;
  box-shadow:
    0 0 20px rgba($primary-color, 0.6),
    inset 0 0 20px rgba($primary-color, 0.2);
  animation: pulse 2s infinite;
  pointer-events: none;
  z-index: 1;
}

@keyframes pulse {
  0%, 100% {
    box-shadow:
      0 0 20px rgba($primary-color, 0.6),
      inset 0 0 20px rgba($primary-color, 0.2);
    border-color: $primary-color;
  }
  50% {
    box-shadow:
      0 0 30px rgba($primary-color, 0.8),
      inset 0 0 30px rgba($primary-color, 0.3);
    border-color: rgba($primary-color, 0.8);
  }
}

.hint-message {
  position: absolute;
  background: #333;
  color: white;
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 14px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  animation: fadeInUp 0.3s ease-out;
  pointer-events: none;
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

@keyframes fadeInUp {
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
