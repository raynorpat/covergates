<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Build } from '@/types/build'
import { formatPercent, coverageColor } from '@/lib/coverage'

const props = defineProps<{ builds: Build[]; repoRoute: string }>()
const router = useRouter()
const branch = ref<string>('')

const branches = computed(() => {
  const set = new Set(props.builds.map((b) => b.branch).filter(Boolean))
  return Array.from(set).sort()
})

const filtered = computed(() =>
  branch.value ? props.builds.filter((b) => b.branch === branch.value) : props.builds
)

function refLabel(b: Build): string {
  return b.pullRequest > 0 ? `PR #${b.pullRequest} → ${b.branch}` : b.branch
}

function shortSha(sha: string): string {
  return sha ? sha.slice(0, 8) : ''
}

function open(b: Build) {
  router.push(`${props.repoRoute}/builds/${b.number}`)
}
</script>

<template>
  <div>
    <v-select
      v-model="branch"
      :items="branches"
      label="Branch"
      density="compact"
      clearable
      hide-details
      style="max-width: 260px"
      class="mb-3"
    />
    <v-table v-if="filtered.length" hover>
      <thead>
        <tr>
          <th>Build</th><th>Ref</th><th>Commit</th><th>Coverage</th><th>Δ</th><th>Status</th><th>When</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="b in filtered" :key="b.number" style="cursor: pointer" @click="open(b)">
          <td>#{{ b.number }}</td>
          <td>{{ refLabel(b) }}</td>
          <td>
            <code>{{ shortSha(b.commit) }}</code>
            <span class="text-medium-emphasis ml-2">{{ b.commitMessage.split('\n')[0] }}</span>
          </td>
          <td>
            <v-chip :color="coverageColor(b.coverage)" size="small" label>{{ formatPercent(b.coverage) }}</v-chip>
          </td>
          <td>
            <span v-if="b.coverageChange > 0" class="text-success">▲ {{ formatPercent(b.coverageChange) }}</span>
            <span v-else-if="b.coverageChange < 0" class="text-error">▼ {{ formatPercent(Math.abs(b.coverageChange)) }}</span>
            <span v-else>—</span>
          </td>
          <td>{{ b.status }}</td>
          <td>{{ new Date(b.createdAt).toLocaleString() }}</td>
        </tr>
      </tbody>
    </v-table>
    <v-alert v-else type="info" variant="tonal">No builds yet.</v-alert>
  </div>
</template>
