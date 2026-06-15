<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { basePath } from '@/lib/base'

const user = useUserStore()
const router = useRouter()
const authed = computed(() => user.isAuthenticated)

function login() { router.push('/login') }
function logout() { window.location.href = basePath() + '/logoff' }
</script>

<template>
  <v-menu v-if="authed">
    <template #activator="{ props }">
      <v-btn icon v-bind="props" aria-label="account">
        <v-avatar size="32">
          <v-img v-if="user.current?.avatar" :src="user.current.avatar" />
          <v-icon v-else>mdi-account-circle</v-icon>
        </v-avatar>
      </v-btn>
    </template>
    <v-list>
      <v-list-item title="Settings" @click="router.push('/user')" />
      <v-list-item title="Logout" @click="logout" />
    </v-list>
  </v-menu>
  <v-btn v-else variant="text" @click="login">Login</v-btn>
</template>
