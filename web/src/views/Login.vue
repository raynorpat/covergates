<script setup lang="ts">
import { useRoute } from 'vue-router'
import { basePath } from '@/lib/base'
import type { SCM } from '@/types'

const route = useRoute()

function loginUrl(scm: SCM): string {
  const redirect = (route.query.redirect as string) || '/'
  const params = new URLSearchParams({ redirect })
  return `${basePath()}/login/${scm}?${params.toString()}`
}

const providers: { scm: SCM; label: string; icon: string }[] = [
  { scm: 'github', label: 'GitHub', icon: 'mdi-github' },
  { scm: 'gitea', label: 'Gitea', icon: 'mdi-git' },
  { scm: 'gitlab', label: 'GitLab', icon: 'mdi-gitlab' }
]
</script>

<template>
  <v-container class="py-12">
    <v-row justify="center">
      <v-col cols="12" sm="6" md="4">
        <v-card class="pa-6 text-center">
          <h2 class="text-h5 mb-6">Sign in</h2>
          <v-btn
            v-for="p in providers"
            :key="p.scm"
            block
            class="mb-3"
            color="primary"
            :prepend-icon="p.icon"
            :href="loginUrl(p.scm)"
          >{{ p.label }}</v-btn>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
