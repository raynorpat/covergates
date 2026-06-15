<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRepositoryStore } from '@/stores/repository'
import { errorMessage } from '@/plugins/http'
import RepoList from '@/components/RepoList.vue'

const store = useRepositoryStore()
const search = ref('')
const busy = ref(false)
const snackbar = ref('')

const filtered = computed(() => {
  const q = search.value.toLowerCase()
  return store.list
    .filter((r) => `${r.NameSpace}/${r.Name}`.toLowerCase().includes(q))
    .sort((a, b) => Number(Boolean(b.ReportID)) - Number(Boolean(a.ReportID)))
})

async function load() {
  busy.value = true
  try { await store.fetchList() } catch (e) { snackbar.value = errorMessage(e) } finally { busy.value = false }
}

async function sync() {
  busy.value = true
  try { await store.synchronize() } catch (e) { snackbar.value = errorMessage(e) } finally { busy.value = false }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <div class="d-flex align-center mb-4 ga-3">
      <v-text-field v-model="search" label="Search repositories" density="compact" hide-details prepend-inner-icon="mdi-magnify" />
      <v-btn color="primary" :loading="busy" prepend-icon="mdi-sync" @click="sync">Sync</v-btn>
    </div>
    <v-progress-linear v-if="busy" indeterminate color="primary" class="mb-2" />
    <v-card>
      <RepoList :repos="filtered" @activated="load" @error="(m) => (snackbar = m)" />
    </v-card>
    <v-snackbar :model-value="!!snackbar" :timeout="4000" color="error" @update:model-value="snackbar = ''">{{ snackbar }}</v-snackbar>
  </v-container>
</template>
