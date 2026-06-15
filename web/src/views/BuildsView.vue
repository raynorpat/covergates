<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import BuildList from '@/components/BuildList.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')
const needLogin = ref(false)

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const repoRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}`)
const signInUrl = computed(() => loginUrl(route.fullPath))

async function load() {
  loading.value = true
  error.value = ''
  needLogin.value = false
  try {
    await store.fetchList(repoPath.value)
  } catch (e) {
    if (is401(e)) needLogin.value = true
    else error.value = errorMessage(e)
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
    <v-card v-else-if="needLogin" class="pa-6 text-center">
      <p class="mb-4">Sign in to view this repository.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <BuildList v-else :builds="store.list" :repo-route="repoRoute" />
  </v-container>
</template>
