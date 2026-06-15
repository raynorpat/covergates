<script setup lang="ts">
import { computed } from 'vue'
import { basePath } from '@/lib/base'

const endpoint = computed(() => `${window.location.origin + basePath()}/api/v1`)
const snippet = computed(() =>
  `# CI: upload coverage with any coveralls reporter\n` +
  `export COVERALLS_ENDPOINT="${endpoint.value}"\n` +
  `export COVERALLS_REPO_TOKEN="<your repo token>"\n` +
  `coveralls report coverage.out   # or your language's coveralls uploader`
)
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Uploading coverage</div>
    <p class="text-body-2 mb-2">
      Point any coveralls-compatible reporter at this server. Set the endpoint to this
      instance and use the repository upload token above.
    </p>
    <v-sheet color="grey-lighten-4" class="pa-3" rounded>
      <pre style="white-space: pre-wrap; margin: 0">{{ snippet }}</pre>
    </v-sheet>
  </v-card>
</template>
