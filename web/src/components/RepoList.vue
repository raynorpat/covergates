<script setup lang="ts">
import RepoListItem from '@/components/RepoListItem.vue'
import type { Repository } from '@/types'

defineProps<{ repos: Repository[] }>()
const emit = defineEmits<{ (e: 'activated'): void; (e: 'error', msg: string): void }>()
</script>

<template>
  <v-list lines="two">
    <RepoListItem
      v-for="repo in repos"
      :key="repo.SCM + repo.NameSpace + repo.Name"
      :repo="repo"
      @activated="emit('activated')"
      @error="(m) => emit('error', m)"
    />
  </v-list>
</template>
