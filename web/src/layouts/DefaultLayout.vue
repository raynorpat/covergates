<script setup lang="ts">
import { useRouter } from 'vue-router'
import AccountButton from '@/components/AccountButton.vue'
const router = useRouter()
</script>

<template>
  <v-app>
    <v-app-bar color="primary" density="comfortable" flat>
      <v-app-bar-title style="cursor: pointer" @click="router.push('/')">Covergates</v-app-bar-title>
      <v-btn variant="text" @click="router.push('/repos')">Repositories</v-btn>
      <AccountButton />
    </v-app-bar>
    <v-main>
      <router-view />
    </v-main>
    <v-footer color="secondary" class="justify-center text-caption">
      <span>Covergates</span>
    </v-footer>
  </v-app>
</template>
