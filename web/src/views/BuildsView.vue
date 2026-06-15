<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { useBuildStore } from '@/stores/build'
import BuildList from '@/components/BuildList.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const repoRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}`)

async function load() {
  loading.value = true
  error.value = ''
  try {
    await store.fetchList(repoPath.value)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h2 class="text-h5 mb-4">{{ namespace }}/{{ name }}</h2>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-alert v-if="error" type="warning" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <BuildList v-else :builds="store.list" :repo-route="repoRoute" />
  </v-container>
</template>
