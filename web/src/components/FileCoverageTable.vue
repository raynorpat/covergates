<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Build, SourceFile } from '@/types/build'
import { fileCoverage, formatPercent, coverageColor } from '@/lib/coverage'

const props = defineProps<{ build: Build; base: Build | null; buildRoute: string }>()
const router = useRouter()
const search = ref('')

interface Row { name: string; covered: number; relevant: number; ratio: number; delta: number | null }

const baseRatios = computed(() => {
  const m = new Map<string, number>()
  for (const f of props.base?.sourceFiles ?? []) m.set(f.name, fileCoverage(f).ratio)
  return m
})

const rows = computed<Row[]>(() => {
  const files: SourceFile[] = props.build.sourceFiles ?? []
  return files
    .map((f) => {
      const c = fileCoverage(f)
      let delta: number | null = null
      if (props.base) delta = c.ratio - (baseRatios.value.get(f.name) ?? 0)
      return { name: f.name, covered: c.covered, relevant: c.relevant, ratio: c.ratio, delta }
    })
    .filter((r) => r.name.toLowerCase().includes(search.value.toLowerCase()))
    .sort((a, b) => a.name.localeCompare(b.name))
})

function openSource(name: string) {
  router.push(`${props.buildRoute}/source/${name}`)
}
</script>

<template>
  <v-card>
    <v-text-field v-model="search" label="Filter files" density="compact" hide-details prepend-inner-icon="mdi-magnify" class="pa-2" />
    <v-table hover>
      <thead>
        <tr><th>File</th><th>Coverage</th><th>Lines</th><th>Δ</th></tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.name" style="cursor: pointer" @click="openSource(r.name)">
          <td><code>{{ r.name }}</code></td>
          <td>
            <v-chip :color="coverageColor(r.ratio)" size="small" label>{{ formatPercent(r.ratio) }}</v-chip>
          </td>
          <td>{{ r.covered }}/{{ r.relevant }}</td>
          <td>
            <span v-if="r.delta === null">—</span>
            <span v-else-if="r.delta > 0" class="text-success">+{{ formatPercent(r.delta) }}</span>
            <span v-else-if="r.delta < 0" class="text-error">{{ formatPercent(r.delta) }}</span>
            <span v-else>0.0%</span>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
