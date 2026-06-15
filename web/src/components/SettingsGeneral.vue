<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const isProtected = ref(props.modelValue.protected)
const mergePR = ref(props.modelValue.mergePR)
const filtersText = ref((props.modelValue.filters ?? []).join('\n'))

watch(() => props.modelValue, (v) => {
  isProtected.value = v.protected
  mergePR.value = v.mergePR
  filtersText.value = (v.filters ?? []).join('\n')
})

function save() {
  emit('save', {
    ...props.modelValue,
    protected: isProtected.value,
    mergePR: mergePR.value,
    filters: filtersText.value.split('\n').map((s) => s.trim()).filter(Boolean)
  })
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">General</div>
    <v-switch v-model="isProtected" label="Protected (only authorized users may upload)" color="primary" hide-details />
    <v-switch v-model="mergePR" label="Auto-merge with previous coverage" color="primary" hide-details class="mb-2" />
    <v-textarea v-model="filtersText" label="File-name filters (one regex per line)" rows="3" auto-grow />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
