<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { useBuildStore } from '@/stores/build'
import BuildSummary from '@/components/BuildSummary.vue'
import FileCoverageTable from '@/components/FileCoverageTable.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const number = computed(() => Number(route.params.number))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const buildRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}/builds/${number.value}`)

async function load() {
  loading.value = true
  error.value = ''
  try {
    await store.fetchBuild(repoPath.value, number.value)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(number, load)
</script>

<template>
  <v-container class="py-6">
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-alert v-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <template v-else-if="store.current">
      <BuildSummary :build="store.current" />
      <FileCoverageTable :build="store.current" :base="store.base" :build-route="buildRoute" />
    </template>
  </v-container>
</template>
