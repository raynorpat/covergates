<script setup lang="ts">
import type { Build } from '@/types/build'
import { formatPercent, coverageColor } from '@/lib/coverage'

defineProps<{ build: Build }>()
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="d-flex align-center ga-4">
      <v-progress-circular :model-value="build.coverage * 100" :color="coverageColor(build.coverage)" size="72" width="8">
        {{ formatPercent(build.coverage) }}
      </v-progress-circular>
      <div>
        <div class="text-h6">Build #{{ build.number }} · {{ build.branch }}</div>
        <div class="text-body-2">
          <code>{{ build.commit.slice(0, 8) }}</code> {{ build.commitMessage.split('\n')[0] }}
        </div>
        <div class="text-medium-emphasis text-body-2">
          {{ build.authorName }} · {{ new Date(build.createdAt).toLocaleString() }} ·
          {{ build.jobs?.length || 0 }} job(s) · {{ build.status }}
          <span v-if="build.baseBuildNumber > 0">
            ·
            <span :class="build.coverageChange >= 0 ? 'text-success' : 'text-error'">
              {{ build.coverageChange >= 0 ? '+' : '' }}{{ formatPercent(build.coverageChange) }}
            </span>
            vs #{{ build.baseBuildNumber }}
          </span>
        </div>
      </div>
    </div>
  </v-card>
</template>
