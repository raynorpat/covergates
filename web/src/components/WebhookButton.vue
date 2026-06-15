<script setup lang="ts">
import { ref } from 'vue'
import http, { errorMessage } from '@/plugins/http'

const props = defineProps<{ repoPath: string }>()
const busy = ref(false)
const message = ref('')

async function create() {
  busy.value = true
  message.value = ''
  try {
    await http.post(`${props.repoPath}/hook/create`)
    message.value = 'Webhook installed.'
  } catch (e) {
    message.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Webhook</div>
    <p class="text-body-2 mb-3">Install a webhook so pull requests get coverage comments and status.</p>
    <v-btn color="primary" :loading="busy" @click="create">Install webhook</v-btn>
    <span v-if="message" class="ml-3 text-body-2">{{ message }}</span>
  </v-card>
</template>
