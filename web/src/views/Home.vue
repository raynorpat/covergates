<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<template>
  <v-container class="py-12">
    <v-row justify="center">
      <v-col cols="12" md="8" class="text-center">
        <h1 class="text-h3 mb-4">Covergates</h1>
        <p class="text-body-1 mb-8">Self-hosted coverage reports, coveralls-compatible.</p>
        <v-btn color="primary" size="large" @click="router.push('/repos')">Get started</v-btn>
      </v-col>
    </v-row>
  </v-container>
</template>
