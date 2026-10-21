<script setup lang="ts">
import { computed, ref } from 'vue'
import ItemSlot from './ItemSlot.vue'
import McTextField from './McTextField.vue'
import { assets } from '../stores/assets'

const model = defineModel<string>({ required: true })
const emit = defineEmits<{ picked: [{ id: string; name: string }] }>()
const query = ref('')

const results = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return []
  const out = []
  for (const item of assets.items.values()) {
    if (item.id.includes(q) || item.name.toLowerCase().includes(q)) out.push(item)
    if (out.length >= 36) break
  }
  return out
})

function pick(id: string, name: string) {
  model.value = id
  emit('picked', { id, name })
  query.value = ''
}
</script>

<template>
  <div class="picker">
    <McTextField v-model="query" type="search" placeholder="Find a texture, e.g. golden apple" aria-label="Find a texture" />
    <p v-if="assets.missing" class="hint">Textures have not been extracted yet, type the item id instead.</p>
    <p v-else-if="query && results.length === 0" class="hint">No texture matches "{{ query }}".</p>
    <div v-if="results.length" class="grid">
      <ItemSlot
        v-for="r in results"
        :key="r.id"
        :texture="r.id"
        interactive
        :selected="r.id === model"
        :label="`${r.name} (${r.id})`"
        :title="`${r.name} (${r.id})`"
        @click="pick(r.id, r.name)"
      />
    </div>
  </div>
</template>

<style scoped>
.picker {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, calc(var(--u) * 18));
}

.hint {
  margin: 0;
  color: var(--dark-gray);
}
</style>
