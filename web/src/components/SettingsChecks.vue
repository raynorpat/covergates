<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const minimum = ref(props.modelValue.coverageMinimum ?? 0)
const decrease = ref(props.modelValue.coverageDecreaseThreshold ?? 0)

watch(() => props.modelValue, (v) => {
  minimum.value = v.coverageMinimum ?? 0
  decrease.value = v.coverageDecreaseThreshold ?? 0
})

function save() {
  emit('save', {
    ...props.modelValue,
    coverageMinimum: Number(minimum.value) || 0,
    coverageDecreaseThreshold: Number(decrease.value) || 0
  })
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Checks</div>
    <p class="text-body-2 text-medium-emphasis mb-3">
      Set the commit-status policy posted to pull requests. Use 0 to disable a check.
    </p>
    <v-text-field
      v-model.number="minimum"
      type="number"
      label="Minimum coverage % (0 = no minimum)"
      density="compact"
    />
    <v-text-field
      v-model.number="decrease"
      type="number"
      label="Max coverage decrease % (0 = any decrease fails)"
      density="compact"
    />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
