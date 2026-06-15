<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import { formatPercent, coverageColor } from '@/lib/coverage'
import type { Build } from '@/types/build'

const route = useRoute()
const router = useRouter()
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

const branches = computed<Build[]>(() => {
  const latest = new Map<string, Build>()
  for (const b of store.list) {
    const cur = latest.get(b.branch)
    if (!cur || b.number > cur.number) latest.set(b.branch, b)
  }
  return Array.from(latest.values()).sort((a, b) => a.branch.localeCompare(b.branch))
})

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
    <h2 class="text-h5 mb-4">{{ namespace }}/{{ name }} · Branches</h2>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-card v-else-if="needLogin" class="pa-6 text-center">
      <p class="mb-4">Sign in to view this repository.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <v-table v-else hover>
      <thead><tr><th>Branch</th><th>Coverage</th><th>Build</th></tr></thead>
      <tbody>
        <tr v-for="b in branches" :key="b.branch" style="cursor: pointer" @click="router.push(`${repoRoute}/builds/${b.number}`)">
          <td>{{ b.branch }}</td>
          <td><v-chip :color="coverageColor(b.coverage)" size="small" label>{{ formatPercent(b.coverage) }}</v-chip></td>
          <td>#{{ b.number }}</td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>
