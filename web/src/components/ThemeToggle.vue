<script setup lang="ts">
import { computed } from 'vue'
import { useThemeStore } from '@/stores/theme'
import AppIcon from './AppIcon.vue'

const theme = useThemeStore()
const icon = computed(() => (theme.preference === 'light' ? 'sun' : theme.preference === 'dark' ? 'moon' : 'monitor'))
const label = computed(() => `Theme: ${theme.preference}. Switch theme`)
</script>

<template>
  <button type="button" class="btn btn-ghost px-2" :aria-label="label" :title="label" @click="theme.cycle()">
    <AppIcon :name="icon" />
  </button>
</template>
