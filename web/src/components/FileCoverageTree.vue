<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Build } from '@/types/build'
import { fileCoverage, formatPercent, coverageColor } from '@/lib/coverage'
import { buildTree, type TreeNode } from '@/lib/fileTree'

const props = defineProps<{ build: Build; buildRoute: string }>()
const router = useRouter()

const root = computed(() =>
  buildTree((props.build.sourceFiles ?? []).map((f) => {
    const c = fileCoverage(f)
    return { path: f.name, covered: c.covered, relevant: c.relevant }
  }))
)

interface Row { node: TreeNode; depth: number }
const rows = computed<Row[]>(() => {
  const out: Row[] = []
  const walk = (node: TreeNode, depth: number) => {
    for (const child of node.children) {
      out.push({ node: child, depth })
      if (child.isDir) walk(child, depth + 1)
    }
  }
  walk(root.value, 0)
  return out
})

function ratio(n: TreeNode): number {
  return n.relevant === 0 ? 0 : n.covered / n.relevant
}
function openSource(path: string) {
  router.push(`${props.buildRoute}/source/${path}`)
}
</script>

<template>
  <v-card>
    <v-list density="compact">
      <v-list-item
        v-for="r in rows"
        :key="r.node.path"
        :style="{ paddingLeft: `${16 + r.depth * 16}px`, cursor: r.node.isDir ? 'default' : 'pointer' }"
        @click="!r.node.isDir && openSource(r.node.path)"
      >
        <template #prepend>
          <v-icon size="small">{{ r.node.isDir ? 'mdi-folder' : 'mdi-file-document-outline' }}</v-icon>
        </template>
        <v-list-item-title>{{ r.node.name }}</v-list-item-title>
        <template #append>
          <v-chip :color="coverageColor(ratio(r.node))" size="x-small" label>{{ formatPercent(ratio(r.node)) }}</v-chip>
        </template>
      </v-list-item>
    </v-list>
    <v-alert v-if="!rows.length" type="info" variant="tonal">No files.</v-alert>
  </v-card>
</template>
