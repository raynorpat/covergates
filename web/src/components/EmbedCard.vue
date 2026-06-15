<script setup lang="ts">
import { computed } from 'vue'
import { basePath } from '@/lib/base'

const props = defineProps<{ reportId: string; repoRoute: string }>()

const origin = computed(() => window.location.origin + basePath())
const badgeUrl = computed(() => `${origin.value}/api/v1/reports/${props.reportId}/badge`)
const cardUrl = computed(() => `${origin.value}/api/v1/reports/${props.reportId}/card`)
const reportUrl = computed(() => `${origin.value}${props.repoRoute}`)
const badgeMd = computed(() => `[![Coverage](${badgeUrl.value})](${reportUrl.value})`)
const cardMd = computed(() => `[![Coverage](${cardUrl.value})](${reportUrl.value})`)

function copy(text: string) {
  navigator.clipboard?.writeText(text)
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Embed</div>
    <img :src="badgeUrl" alt="coverage badge" class="mb-3" />
    <v-text-field :model-value="badgeMd" label="Badge markdown" readonly density="compact"
      append-inner-icon="mdi-content-copy" @click:append-inner="copy(badgeMd)" />
    <v-text-field :model-value="cardMd" label="Card markdown" readonly density="compact"
      append-inner-icon="mdi-content-copy" @click:append-inner="copy(cardMd)" />
  </v-card>
</template>
