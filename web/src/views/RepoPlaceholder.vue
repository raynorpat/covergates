<script setup lang="ts">
import { useRoute } from 'vue-router'
const route = useRoute()
</script>

<template>
  <v-container class="py-12 text-center">
    <h2 class="text-h5 mb-2">{{ route.params.namespace }}/{{ route.params.name }}</h2>
    <p class="text-body-2">Build reports for this repository are coming soon.</p>
  </v-container>
</template>
