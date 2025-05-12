<template>
  <div class="not-found">
    <h1>404 - Page Not Found</h1>
    <p>The page you are looking for does not exist.</p>
    <p>Current path: {{ $route.path }}</p>
    <button @click="goHome">Go Home</button>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

function goHome() {
  router.push('/')
}
</script>

<style lang="scss" scoped>
@import '../assets/main.scss';

.not-found {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
  background-color: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  text-align: center;
}

h1 {
  color: $danger-color;
}

button {
  background-color: $primary-color;
  color: white;
  padding: 8px 16px;
  border: none;
  border-radius: $default-radius;
  cursor: pointer;
  margin-top: 20px;

  &:hover {
    background-color: $primary-hover-color;
  }
}
</style>
