<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useUserStore } from '@/stores/user'
import { basePath } from '@/lib/base'

const user = useUserStore()
onMounted(() => user.fetchScm())

const providers = [
  { scm: 'github', label: 'GitHub', icon: 'mdi-github' },
  { scm: 'gitea', label: 'Gitea', icon: 'mdi-git' },
  { scm: 'gitlab', label: 'GitLab', icon: 'mdi-gitlab' }
]

const rows = computed(() => providers.map((p) => ({ ...p, linked: !!user.scm[p.scm] })))
function linkUrl(scm: string) {
  return `${basePath()}/login/${scm}?bind`
}
</script>

<template>
  <v-card class="pa-4">
    <div class="text-h6 mb-2">Linked accounts</div>
    <v-list>
      <v-list-item v-for="p in rows" :key="p.scm" :title="p.label" :prepend-icon="p.icon">
        <template #append>
          <v-icon v-if="p.linked" color="success" aria-label="linked">mdi-check-circle</v-icon>
          <v-btn v-else size="small" variant="tonal" :href="linkUrl(p.scm)">Link</v-btn>
        </template>
      </v-list-item>
    </v-list>
  </v-card>
</template>
