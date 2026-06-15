<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useUserStore } from '@/stores/user'
import { basePath } from '@/lib/base'

const user = useUserStore()
onMounted(() => user.fetchScm())

const META: Record<string, { label: string; icon: string }> = {
  github: { label: 'GitHub', icon: 'mdi-github' },
  gitea: { label: 'Gitea', icon: 'mdi-git' },
  gitlab: { label: 'GitLab', icon: 'mdi-gitlab' }
}

const rows = computed(() =>
  Object.entries(user.scm).map(([scm, linked]) => ({
    scm,
    linked,
    label: META[scm]?.label ?? scm,
    icon: META[scm]?.icon ?? 'mdi-git'
  }))
)

// The backend binds the single configured SCM to the current account via /login?bind.
const bindUrl = `${basePath()}/login?bind`
</script>

<template>
  <v-card class="pa-4 h-100">
    <div class="text-h6 mb-2">Linked accounts</div>
    <v-list>
      <v-list-item v-for="p in rows" :key="p.scm" :title="p.label" :prepend-icon="p.icon">
        <template #append>
          <v-icon v-if="p.linked" color="success" aria-label="linked">mdi-check-circle</v-icon>
          <v-btn v-else size="small" variant="tonal" :href="bindUrl">Link</v-btn>
        </template>
      </v-list-item>
    </v-list>
  </v-card>
</template>
