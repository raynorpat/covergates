<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { basePath } from '@/lib/base'

const route = useRoute()

// The backend serves a single configured SCM at /login and auto-redirects to it.
const signInUrl = computed(() => {
  const redirect = (route.query.redirect as string) || '/'
  return `${basePath()}/login?${new URLSearchParams({ redirect }).toString()}`
})
</script>

<template>
  <v-container class="py-12">
    <v-row justify="center">
      <v-col cols="12" sm="6" md="4">
        <v-card class="pa-6 text-center">
          <h2 class="text-h5 mb-6">Sign in</h2>
          <v-btn block color="primary" prepend-icon="mdi-git" :href="signInUrl">
            Sign in with your Git provider
          </v-btn>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
