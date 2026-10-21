<script setup lang="ts">
import SiteFrame from '../components/SiteFrame.vue'
import McButton from '../components/McButton.vue'
import { usePageTitle } from '../lib/title'

usePageTitle(() => 'Not found')
</script>

<template>
  <SiteFrame>
    <div class="nf shadow">
      <p>This page does not exist. Maybe a creeper got to it.</p>
      <McButton to="/">Back to shop</McButton>
    </div>
  </SiteFrame>
</template>

<style scoped>
.nf {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--u) * 6);
  text-align: center;
}
</style>
