<script setup lang="ts">
import McTooltip from './components/McTooltip.vue'
import { loadAssets } from './stores/assets'

loadAssets()
</script>

<template>
  <RouterView />
  <McTooltip />
</template>
