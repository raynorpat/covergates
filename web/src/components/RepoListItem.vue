<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import http, { errorMessage } from '@/plugins/http'
import type { Repository } from '@/types'

const props = defineProps<{ repo: Repository }>()
const emit = defineEmits<{ (e: 'activated'): void; (e: 'error', msg: string): void }>()
const router = useRouter()

const busy = ref(false)
const token = ref('')
const showToken = ref(false)

const repoPath = () => `/api/v1/repos/${props.repo.SCM}/${props.repo.NameSpace}/${props.repo.Name}`

async function activate() {
  busy.value = true
  try {
    try {
      await http.get(repoPath())
    } catch {
      await http.post('/api/v1/repos', props.repo)
    }
    await http.patch(`${repoPath()}/report`)
    emit('activated')
  } catch (e) {
    emit('error', errorMessage(e))
  } finally {
    busy.value = false
  }
}

async function revealToken() {
  busy.value = true
  try {
    const { data } = await http.get<{ token: string }>(`${repoPath()}/token`)
    token.value = data.token
    showToken.value = true
  } catch (e) {
    emit('error', errorMessage(e))
  } finally {
    busy.value = false
  }
}

async function rotateToken() {
  busy.value = true
  try {
    const { data } = await http.patch<{ token: string }>(`${repoPath()}/token`)
    token.value = data.token
    showToken.value = true
  } catch (e) {
    emit('error', errorMessage(e))
  } finally {
    busy.value = false
  }
}

function copyToken() {
  if (token.value) navigator.clipboard?.writeText(token.value)
}

function open() {
  router.push(`/report/${props.repo.SCM}/${props.repo.NameSpace}/${props.repo.Name}`)
}
</script>

<template>
  <v-list-item>
    <template #prepend>
      <v-icon>{{ repo.Private ? 'mdi-lock' : 'mdi-source-repository' }}</v-icon>
    </template>
    <v-list-item-title>{{ repo.NameSpace }}/{{ repo.Name }}</v-list-item-title>
    <v-list-item-subtitle>{{ repo.URL }}</v-list-item-subtitle>

    <template #append>
      <template v-if="repo.ReportID">
        <v-btn variant="text" :loading="busy" @click="showToken ? rotateToken() : revealToken()">
          {{ showToken ? 'Rotate token' : 'Show token' }}
        </v-btn>
        <v-btn icon variant="text" aria-label="open" @click="open"><v-icon>mdi-chevron-right</v-icon></v-btn>
      </template>
      <v-btn v-else color="primary" variant="tonal" :loading="busy" @click="activate">Activate</v-btn>
    </template>
  </v-list-item>

  <v-list-item v-if="showToken">
    <v-text-field
      :model-value="token"
      label="Upload token"
      readonly
      density="compact"
      append-inner-icon="mdi-content-copy"
      @click:append-inner="copyToken"
    />
  </v-list-item>
</template>
