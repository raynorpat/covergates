<script setup lang="ts">
import { computed } from 'vue'
import '@/assets/styles/variables.scss'
import { lineState } from '@/lib/coverage'
import { languageFor, highlightLine } from '@/lib/highlight'

const props = defineProps<{ source: string; coverage: (number | null)[]; path: string }>()

interface Row { n: number; html: string; cls: string; hits: number | null }

const rows = computed<Row[]>(() => {
  const lang = languageFor(props.path)
  const lines = props.source.replace(/\r\n/g, '\n').split('\n')
  return lines.map((line, i) => {
    const hits = i < props.coverage.length ? props.coverage[i] : null
    const state = lineState(hits)
    return {
      n: i + 1,
      html: highlightLine(line, lang),
      cls: state === 'hit' ? 'statement-hit' : state === 'miss' ? 'statement-miss' : '',
      hits
    }
  })
})
</script>

<template>
  <v-card class="pa-0">
    <table class="source">
      <tbody>
        <tr v-for="r in rows" :key="r.n" :class="r.cls">
          <td class="ln">{{ r.n }}</td>
          <td class="hits">{{ r.hits !== null && r.hits !== undefined ? r.hits + '×' : '' }}</td>
          <td class="code"><pre><code v-html="r.html"></code></pre></td>
        </tr>
      </tbody>
    </table>
  </v-card>
</template>

<style scoped>
.source { width: 100%; border-collapse: collapse; font-family: ui-monospace, monospace; font-size: 12px; }
.source td { padding: 0 8px; vertical-align: top; }
.ln { text-align: right; width: 48px; user-select: none; opacity: 0.5; }
.hits { text-align: right; width: 48px; user-select: none; opacity: 0.6; }
.code pre { margin: 0; white-space: pre-wrap; word-break: break-word; }
</style>
