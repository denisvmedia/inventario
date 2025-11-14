<template>
  <div class="error-notification-stack">
    <TransitionGroup name="error-stack" tag="div" class="error-stack-container">
      <div
        v-for="error in errors"
        :key="error.id"
        class="error-notification"
        role="alert"
        aria-live="assertive"
        @touchstart="handleTouchStart($event)"
        @touchmove="handleTouchMove($event)"
        @touchend="handleTouchEnd($event, error.id)"
      >
        <div class="error-content">
          <div class="error-icon">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
            </svg>
          </div>
          <div class="error-message">
            {{ error.message }}
          </div>
          <button
            type="button"
            class="error-dismiss"
            aria-label="Dismiss error"
            @click="$emit('dismiss', error.id)"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
              <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
            </svg>
          </button>
        </div>
        <div v-if="error.context" class="error-context">
          {{ formatContext(error.context) }}
        </div>
        <div class="error-hint mobile-only">
          Tap × or swipe to dismiss
        </div>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
interface ErrorItem {
  id: string;
  message: string;
  timestamp: number;
  context?: string;
}

interface Props {
  errors: ErrorItem[];
}

defineProps<Props>();

const emit = defineEmits<{
  dismiss: [errorId: string];
}>();

const formatContext = (context: string): string => {
  return `Error in ${context} operation`;
};

// Touch handling for swipe-to-dismiss on mobile
let touchStartX = 0;
let touchStartY = 0;
let touchStartTime = 0;

const handleTouchStart = (event: TouchEvent) => {
  const touch = event.touches[0];
  touchStartX = touch.clientX;
  touchStartY = touch.clientY;
  touchStartTime = Date.now();
};

const handleTouchMove = (event: TouchEvent) => {
  // Prevent scrolling when swiping horizontally
  const touch = event.touches[0];
  const deltaX = Math.abs(touch.clientX - touchStartX);
  const deltaY = Math.abs(touch.clientY - touchStartY);

  if (deltaX > deltaY && deltaX > 10) {
    event.preventDefault();
  }
};

const handleTouchEnd = (event: TouchEvent, errorId: string) => {
  const touch = event.changedTouches[0];
  const deltaX = touch.clientX - touchStartX;
  const deltaY = touch.clientY - touchStartY;
  const deltaTime = Date.now() - touchStartTime;

  // Check for horizontal swipe (right or left)
  const isHorizontalSwipe = Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > 50;
  const isQuickSwipe = deltaTime < 300;
  const isFastSwipe = Math.abs(deltaX) > 100;

  if (isHorizontalSwipe && (isQuickSwipe || isFastSwipe)) {
    // Emit dismiss event for swipe-to-dismiss
    emit('dismiss', errorId);
  }
};
</script>

<style scoped>
.error-notification-stack {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 9999;
  max-width: 400px;
  pointer-events: none;
}

.error-stack-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.error-notification {
  background: #fee2e2;
  border: 1px solid #fecaca;
  border-left: 4px solid #dc2626;
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 10px 15px -3px rgb(0 0 0 / 10%), 0 4px 6px -2px rgb(0 0 0 / 5%);
  pointer-events: auto;
  max-width: 100%;
  overflow-wrap: break-word;
}

.error-content {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.error-icon {
  color: #dc2626;
  flex-shrink: 0;
  margin-top: 2px;
}

.error-message {
  flex: 1;
  color: #7f1d1d;
  font-weight: 500;
  line-height: 1.5;
}

.error-dismiss {
  background: none;
  border: none;
  color: #991b1b;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  flex-shrink: 0;
  transition: background-color 0.2s;
}

.error-dismiss:hover {
  background-color: #fecaca;
}

.error-dismiss:focus {
  outline: 2px solid #dc2626;
  outline-offset: 2px;
}

.error-dismiss:active {
  background-color: #f87171;
  transform: scale(0.95);
}

/* Touch-friendly interactions */
@media (hover: none) and (pointer: coarse) {
  .error-dismiss {
    min-width: 44px;
    min-height: 44px;
    padding: 10px;
  }

  .error-dismiss:hover {
    background-color: transparent;
  }

  .error-dismiss:active {
    background-color: #fecaca;
    transform: scale(0.9);
  }
}

.error-context {
  margin-top: 8px;
  font-size: 0.875rem;
  color: #991b1b;
  opacity: 0.8;
}

.error-hint {
  margin-top: 6px;
  font-size: 0.75rem;
  color: #991b1b;
  opacity: 0.6;
  font-style: italic;
}

.mobile-only {
  display: none;
}

/* Transition animations */
.error-stack-enter-active {
  transition: all 0.3s ease-out;
}

.error-stack-leave-active {
  transition: all 0.3s ease-in;
}

.error-stack-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.error-stack-leave-to {
  opacity: 0;
  transform: translateX(100%);
}

.error-stack-move {
  transition: transform 0.3s ease;
}

/* Mobile-first responsive design */
@media (width <= 768px) {
  .error-notification-stack {
    position: fixed;
    inset: 0 0 auto;
    z-index: 9999;
    max-width: none;
    padding: 10px;
    background: rgb(0 0 0 / 2%);
    backdrop-filter: blur(2px);
  }

  .error-stack-container {
    max-height: 50vh;
    overflow-y: auto;
    gap: 8px;
  }

  .error-notification {
    padding: 16px;
    margin: 0;
    border-radius: 12px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 15%);
    border-left-width: 6px;
    font-size: 0.9rem;
    line-height: 1.4;
  }

  .error-content {
    gap: 12px;
    align-items: flex-start;
  }

  .error-icon {
    margin-top: 1px;
    flex-shrink: 0;
  }

  .error-icon svg {
    width: 18px;
    height: 18px;
  }

  .error-message {
    font-size: 0.9rem;
    line-height: 1.4;
    overflow-wrap: break-word;
    hyphens: auto;
  }

  .error-dismiss {
    padding: 8px;
    margin: -4px;
    border-radius: 6px;
    min-width: 32px;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .error-dismiss svg {
    width: 18px;
    height: 18px;
  }

  .error-context {
    margin-top: 6px;
    font-size: 0.8rem;
    line-height: 1.3;
  }
}

/* Small mobile devices */
@media (width <= 480px) {
  .error-notification-stack {
    padding: 8px;
  }

  .error-notification {
    padding: 14px;
    border-radius: 10px;
    font-size: 0.85rem;
  }

  .error-content {
    gap: 10px;
  }

  .error-icon svg {
    width: 16px;
    height: 16px;
  }

  .error-message {
    font-size: 0.85rem;
    line-height: 1.35;
  }

  .error-dismiss {
    padding: 6px;
    min-width: 28px;
    min-height: 28px;
  }

  .error-dismiss svg {
    width: 16px;
    height: 16px;
  }

  .error-context {
    font-size: 0.75rem;
    margin-top: 4px;
  }

  .mobile-only {
    display: block;
  }

  .error-hint {
    font-size: 0.7rem;
    margin-top: 4px;
  }
}

/* Landscape mobile orientation */
@media (width <= 768px) and (orientation: landscape) {
  .error-notification-stack {
    top: 5px;
    padding: 5px;
    background: rgb(0 0 0 / 5%);
  }

  .error-stack-container {
    max-height: 40vh;
  }

  .error-notification {
    padding: 12px;
    border-radius: 8px;
  }

  .error-content {
    gap: 8px;
  }

  .error-message {
    font-size: 0.8rem;
    line-height: 1.3;
  }

  .error-context {
    font-size: 0.7rem;
    margin-top: 3px;
  }
}

/* High contrast mode support */
@media (prefers-contrast: high) {
  .error-notification {
    border-width: 2px;
    border-left-width: 6px;
  }
}

/* Swipe feedback animation */
.error-notification.swiping {
  transform: translateX(var(--swipe-offset, 0));
  transition: none;
}

.error-notification.swipe-dismiss {
  transform: translateX(100%);
  opacity: 0;
  transition: transform 0.3s ease-out, opacity 0.3s ease-out;
}

/* Reduced motion support */
@media (prefers-reduced-motion: reduce) {
  .error-stack-enter-active,
  .error-stack-leave-active,
  .error-stack-move,
  .error-notification.swipe-dismiss {
    transition: none;
  }

  .error-notification.swiping {
    transform: none;
  }
}
</style>
