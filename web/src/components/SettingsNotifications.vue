<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const trigger = ref(props.modelValue.notifyTrigger || 'failure')
const recipientsText = ref((props.modelValue.emailRecipients ?? []).join('\n'))
const slackWebhook = ref(props.modelValue.slackWebhook ?? '')

const triggerItems = [
  { title: 'Off', value: 'off' },
  { title: 'On failure', value: 'failure' },
  { title: 'Always', value: 'always' }
]

watch(() => props.modelValue, (v) => {
  trigger.value = v.notifyTrigger || 'failure'
  recipientsText.value = (v.emailRecipients ?? []).join('\n')
  slackWebhook.value = v.slackWebhook ?? ''
})

function save() {
  emit('save', {
    ...props.modelValue,
    notifyTrigger: trigger.value,
    emailRecipients: recipientsText.value.split('\n').map((s) => s.trim()).filter(Boolean),
    slackWebhook: slackWebhook.value.trim()
  })
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Notifications</div>
    <p class="text-body-2 text-medium-emphasis mb-3">
      Email and Slack delivery for finalized builds.
    </p>
    <v-select v-model="trigger" :items="triggerItems" label="When to notify" density="compact" />
    <v-textarea v-model="recipientsText" label="Email recipients (one per line)" rows="3" auto-grow />
    <v-text-field v-model="slackWebhook" type="text" label="Slack webhook URL" density="compact" />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
