<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import http, { errorMessage } from '@/plugins/http'
import { is401, isStatus, loginUrl } from '@/lib/auth'
import { useRepositoryStore } from '@/stores/repository'
import { useRepoToken } from '@/lib/useRepoToken'
import type { Repository } from '@/types'
import type { RepoSetting } from '@/types/setting'
import SettingsGeneral from '@/components/SettingsGeneral.vue'
import SettingsChecks from '@/components/SettingsChecks.vue'
import SettingsNotifications from '@/components/SettingsNotifications.vue'
import WebhookButton from '@/components/WebhookButton.vue'
import EmbedCard from '@/components/EmbedCard.vue'
import UploadGuide from '@/components/UploadGuide.vue'

const route = useRoute()
const store = useRepositoryStore()
const loading = ref(false)
const error = ref('')
const denied = ref(false)
const saving = ref(false)
const snackbar = ref('')
const repo = ref<Repository | null>(null)

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const repoRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}`)
const signInUrl = computed(() => loginUrl(route.fullPath))

const tokenCtl = useRepoToken(repoPath.value)

async function load() {
  loading.value = true
  error.value = ''
  denied.value = false
  try {
    const res = await http.get<Repository>(repoPath.value)
    repo.value = res.data
    await store.fetchSetting(repoPath.value)
  } catch (e) {
    if (is401(e) || isStatus(e, 403)) denied.value = true
    else error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function save(value: RepoSetting) {
  saving.value = true
  try {
    await store.updateSetting(repoPath.value, value)
    snackbar.value = 'Settings saved.'
  } catch (e) {
    snackbar.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h2 class="text-h5 mb-4">{{ namespace }}/{{ name }} · Settings</h2>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-card v-else-if="denied" class="pa-6 text-center">
      <p class="mb-4">You don't have access to this repository's settings.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <template v-else-if="store.setting && repo">
      <SettingsGeneral :model-value="store.setting" :busy="saving" @save="save" />
      <SettingsChecks :model-value="store.setting" :busy="saving" @save="save" />
      <SettingsNotifications :model-value="store.setting" :busy="saving" @save="save" />
      <WebhookButton :repo-path="repoPath" />
      <v-card class="pa-4 mb-4">
        <div class="text-h6 mb-2">Upload token</div>
        <p class="text-body-2 text-medium-emphasis mb-2">Hidden for security — click to reveal.</p>
        <v-btn variant="text" :loading="tokenCtl.busy.value" @click="tokenCtl.revealed.value ? tokenCtl.rotate() : tokenCtl.reveal()">
          {{ tokenCtl.revealed.value ? 'Rotate token' : 'Show token' }}
        </v-btn>
        <v-text-field v-if="tokenCtl.revealed.value" :model-value="tokenCtl.token.value" label="Upload token" readonly density="compact" class="mt-2" />
      </v-card>
      <EmbedCard :report-id="repo.ReportID" :repo-route="repoRoute" />
      <UploadGuide />
    </template>
    <v-snackbar :model-value="!!snackbar" :timeout="4000" @update:model-value="snackbar = ''">{{ snackbar }}</v-snackbar>
  </v-container>
</template>
