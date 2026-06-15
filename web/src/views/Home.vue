<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useRepositoryStore } from '@/stores/repository'
import { formatPercent, coverageColor } from '@/lib/coverage'

const router = useRouter()
const user = useUserStore()
const repos = useRepositoryStore()
const busy = ref(false)

onMounted(async () => {
  if (!user.current) await user.fetch()
  if (!user.isAuthenticated) return
  busy.value = true
  try {
    await repos.ensureSynced()
    await repos.fetchStats()
  } finally {
    busy.value = false
  }
})

function openRepo(r: { scm: string; namespace: string; name: string }) {
  router.push(`/report/${r.scm}/${r.namespace}/${r.name}/builds`)
}
</script>

<template>
  <v-container class="py-10">
    <!-- Guest hero -->
    <v-row v-if="!user.isAuthenticated" justify="center">
      <v-col cols="12" md="8" class="text-center">
        <h1 class="text-h3 mb-4">Covergates</h1>
        <p class="text-body-1 mb-8">Self-hosted coverage reports, coveralls-compatible.</p>
        <v-btn color="primary" size="large" @click="router.push('/repos')">Get started</v-btn>
      </v-col>
    </v-row>

    <!-- Authenticated dashboard -->
    <template v-else>
      <h1 class="text-h4 mb-6">Welcome, {{ user.current?.login }}</h1>
      <v-progress-linear v-if="busy" indeterminate color="primary" class="mb-4" />
      <v-row v-if="repos.stats" class="mb-2">
        <v-col cols="12" sm="4">
          <v-card class="pa-4 h-100">
            <div class="text-overline">Repositories</div>
            <div class="text-h4">{{ repos.stats.repoCount }}</div>
          </v-card>
        </v-col>
        <v-col cols="12" sm="4">
          <v-card class="pa-4 h-100">
            <div class="text-overline">Activated</div>
            <div class="text-h4">{{ repos.stats.activatedCount }}</div>
          </v-card>
        </v-col>
        <v-col cols="12" sm="4">
          <v-card class="pa-4 h-100">
            <div class="text-overline">Avg coverage</div>
            <div class="text-h4">{{ formatPercent(repos.stats.averageCoverage) }}</div>
          </v-card>
        </v-col>
      </v-row>

      <v-card v-if="repos.stats" class="pa-4">
        <div class="text-h6 mb-2">Top repositories</div>
        <v-table v-if="repos.stats.topRepos.length" density="compact">
          <tbody>
            <tr v-for="r in repos.stats.topRepos" :key="`${r.namespace}/${r.name}`" style="cursor: pointer" @click="openRepo(r)">
              <td><code>{{ r.namespace }}/{{ r.name }}</code></td>
              <td class="text-right">
                <v-chip :color="coverageColor(r.coverage)" size="small" label>{{ formatPercent(r.coverage) }}</v-chip>
              </td>
            </tr>
          </tbody>
        </v-table>
        <p v-else class="text-body-2 text-medium-emphasis mb-0">
          No coverage yet — <a href="#" @click.prevent="router.push('/repos')">activate a repository</a>.
        </p>
      </v-card>
    </template>
  </v-container>
</template>
