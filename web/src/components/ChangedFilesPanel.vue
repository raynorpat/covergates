<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import { fileCoverage, formatPercent, coverageColor } from '@/lib/coverage'
import type { Build, FileChange } from '@/types/build'

const props = defineProps<{ build: Build; repoPath: string; buildRoute: string; loginRedirect: string }>()
const router = useRouter()
const store = useBuildStore()

const loading = ref(false)
const needLogin = ref(false)
const error = ref('')
const changes = ref<FileChange[]>([])

const coverageByPath = computed(() => {
  const m = new Map<string, number | null>()
  for (const f of props.build.sourceFiles ?? []) {
    const c = fileCoverage(f)
    m.set(f.name, c.relevant === 0 ? null : c.ratio)
  }
  return m
})

async function load() {
  loading.value = true
  needLogin.value = false
  error.value = ''
  try {
    changes.value = await store.fetchChanges(props.repoPath, props.build.number)
  } catch (e) {
    if (is401(e)) needLogin.value = true
    else error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function ratioOf(path: string): number | null | undefined {
  return coverageByPath.value.get(path)
}
function openSource(path: string) {
  router.push(`${props.buildRoute}/source/${path}`)
}

onMounted(load)
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Changed files (PR #{{ build.pullRequest }})</div>
    <v-progress-linear v-if="loading" indeterminate color="primary" />
    <p v-else-if="needLogin" class="text-body-2">
      <a :href="loginUrl(loginRedirect)">Sign in</a> to view changed files.
    </p>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <v-table v-else density="compact">
      <tbody>
        <tr v-for="ch in changes" :key="ch.path" :style="{ cursor: ch.deleted ? 'default' : 'pointer' }" @click="!ch.deleted && openSource(ch.path)">
          <td :class="{ 'text-decoration-line-through': ch.deleted }"><code>{{ ch.path }}</code></td>
          <td>
            <span v-if="ch.deleted" class="text-medium-emphasis">deleted</span>
            <v-chip v-else-if="ratioOf(ch.path) != null" :color="coverageColor(ratioOf(ch.path) as number)" size="x-small" label>
              {{ formatPercent(ratioOf(ch.path) as number) }}
            </v-chip>
            <span v-else class="text-medium-emphasis">no coverage data</span>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
