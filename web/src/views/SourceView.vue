<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import SourceLines from '@/components/SourceLines.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')
const needLogin = ref(false)
const source = ref('')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const number = computed(() => Number(route.params.number))
const path = computed(() => Array.isArray(route.params.path) ? route.params.path.join('/') : String(route.params.path))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const signInUrl = computed(() => loginUrl(route.fullPath))

const coverage = computed<(number | null)[]>(() => {
  const f = store.current?.sourceFiles?.find((s) => s.name === path.value)
  return f?.coverage ?? []
})

async function load() {
  loading.value = true
  error.value = ''
  needLogin.value = false
  try {
    if (!store.current || store.current.number !== number.value) {
      await store.fetchBuild(repoPath.value, number.value)
    }
    const commit = store.current?.commit ?? ''
    source.value = await store.fetchSource(repoPath.value, path.value, commit)
  } catch (e) {
    if (is401(e)) {
      needLogin.value = true
    } else {
      error.value = errorMessage(e)
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h3 class="text-subtitle-1 mb-3"><code>{{ path }}</code> · build #{{ number }}</h3>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-card v-else-if="needLogin" class="pa-6 text-center">
      <p class="mb-4">Sign in to view source for this repository.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <SourceLines v-else :source="source" :coverage="coverage" :path="path" />
  </v-container>
</template>
