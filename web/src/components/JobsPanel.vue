<script setup lang="ts">
import type { Job } from '@/types/build'
import { formatPercent, coverageColor } from '@/lib/coverage'

defineProps<{ jobs: Job[] }>()
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Jobs</div>
    <v-table density="compact">
      <thead>
        <tr><th>Job</th><th>Flag</th><th>Coverage</th></tr>
      </thead>
      <tbody>
        <tr v-for="(job, i) in jobs" :key="job.id || i">
          <td>{{ job.serviceJobID || ('#' + (i + 1)) }}</td>
          <td>{{ job.flag || '—' }}</td>
          <td><v-chip :color="coverageColor(job.coverage)" size="x-small" label>{{ formatPercent(job.coverage) }}</v-chip></td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
