<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
</script>

<template>
  <div class="card">
    <h1 id="page-title" tabindex="-1" class="sr-only">Page not found</h1>
    <EmptyState title="Page not found" icon="search">
      The page you were looking for does not exist.
      <template #actions><RouterLink to="/" class="btn btn-primary">Go to dashboard</RouterLink></template>
    </EmptyState>
  </div>
</template>
