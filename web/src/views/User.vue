<script setup lang="ts">
import { onMounted } from 'vue'
import { useUserStore } from '@/stores/user'

const user = useUserStore()
onMounted(() => { if (!user.current) user.fetch() })
</script>

<template>
  <v-container class="py-8">
    <v-row justify="center">
      <v-col cols="12" md="6">
        <v-card class="pa-6">
          <div class="d-flex align-center mb-4">
            <v-avatar size="64" class="mr-4">
              <v-img v-if="user.current?.avatar" :src="user.current.avatar" />
              <v-icon v-else size="64">mdi-account-circle</v-icon>
            </v-avatar>
            <div>
              <div class="text-h6">{{ user.current?.login }}</div>
              <div class="text-body-2">{{ user.current?.email }}</div>
            </div>
          </div>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
