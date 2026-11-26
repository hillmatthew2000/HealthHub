<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
</script>

<template>
  <div class="card">
    <h1 id="page-title" tabindex="-1" class="sr-only">No access</h1>
    <EmptyState title="You do not have access to this page" icon="shield">
      Your role does not include this area. If you need it, ask an administrator.
      <template #actions><RouterLink to="/" class="btn btn-primary">Go to dashboard</RouterLink></template>
    </EmptyState>
  </div>
</template>
