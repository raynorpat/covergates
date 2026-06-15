<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, isStatus, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import BuildSummary from '@/components/BuildSummary.vue'
import FileCoverageTable from '@/components/FileCoverageTable.vue'
import FileCoverageTree from '@/components/FileCoverageTree.vue'
import JobsPanel from '@/components/JobsPanel.vue'
import ChangedFilesPanel from '@/components/ChangedFilesPanel.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')
const needLogin = ref(false)
const notFound = ref(false)
const fileView = ref<'flat' | 'tree'>('flat')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const number = computed(() => Number(route.params.number))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const buildRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}/builds/${number.value}`)
const signInUrl = computed(() => loginUrl(route.fullPath))

async function load() {
  loading.value = true
  error.value = ''
  needLogin.value = false
  notFound.value = false
  try {
    await store.fetchBuild(repoPath.value, number.value)
  } catch (e) {
    if (is401(e)) needLogin.value = true
    else if (isStatus(e, 404)) notFound.value = true
    else error.value = errorMessage(e)
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
    <v-card v-else-if="needLogin" class="pa-6 text-center">
      <p class="mb-4">Sign in to view this repository.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="notFound" type="info" variant="tonal">Build not found.</v-alert>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <template v-else-if="store.current">
      <BuildSummary :build="store.current" />
      <JobsPanel v-if="store.current.jobs && store.current.jobs.length" :jobs="store.current.jobs" />
      <ChangedFilesPanel
        v-if="store.current.pullRequest > 0"
        :build="store.current"
        :repo-path="repoPath"
        :build-route="buildRoute"
        :login-redirect="$route.fullPath"
      />
      <div class="d-flex justify-end mb-2">
        <v-btn-toggle v-model="fileView" density="compact" mandatory>
          <v-btn value="flat" size="small">Flat</v-btn>
          <v-btn value="tree" size="small">Tree</v-btn>
        </v-btn-toggle>
      </div>
      <FileCoverageTable v-if="fileView === 'flat'" :build="store.current" :base="store.base" :build-route="buildRoute" />
      <FileCoverageTree v-else :build="store.current" :build-route="buildRoute" />
    </template>
  </v-container>
</template>
