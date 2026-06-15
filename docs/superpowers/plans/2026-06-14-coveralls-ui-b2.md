# Coveralls UI B2 — Coverage Browsing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the coveralls-style coverage-browsing UI on the Build API — builds list (repo page), build detail (summary + per-file table with per-file deltas), and per-line source view — on the Vue 3 stack from B1.

**Architecture:** New Vue 3 `<script setup>` views/components + a Pinia `build` store + pure coverage/highlight utils, all under `web/src`. One small additive backend change exposes the base build's number so the frontend can compute per-file deltas. Build list/detail are public (API enforces per-repo access); the per-line source view requires login (content endpoint is auth-gated) and shows a sign-in prompt otherwise.

**Tech Stack:** Vue 3.5, Vuetify 3, Vue Router 4, Pinia, Vite, Vitest, highlight.js 11, Axios. Go 1.14 backend. Node 24/npm available locally.

**Conventions (CLAUDE.md):**
- Commit messages: short, subsystem-prefixed (`web:` for frontend, `build:`/`core:` for backend), no metadata/trailers.
- Surgical; match existing patterns; simplicity first.
- Frontend commands from `D:/raynorpat/covergates/web`; Go commands from repo root.
- Backend build uses explicit package paths (NOT `go build ./...`, which walks `web/node_modules`): `go build ./core/... ./models/... ./modules/... ./routers/...`.
- A gitignored `web/web_gen.go` stub exists locally; never commit it, `web/dist`, or `web/node_modules`.

**Build verification (frontend):** `npm run build` runs `vue-tsc --noEmit && vite build` — run it (not just vitest) to catch type errors. `tsconfig.json` already includes `src/**`.

---

## File structure

Backend (modify): `core/build.go`, `models/build.go`, `modules/build/service.go`, `modules/build/service_test.go`, `models/build_test.go`.

Frontend (create):
- `web/src/types/build.ts`
- `web/src/lib/coverage.ts` (+ `__tests__/coverage.spec.ts`)
- `web/src/lib/highlight.ts` (+ `__tests__/highlight.spec.ts`)
- `web/src/stores/build.ts` (+ `__tests__/build.spec.ts`)
- `web/src/components/BuildList.vue` (+ test)
- `web/src/components/BuildSummary.vue`
- `web/src/components/FileCoverageTable.vue` (+ test)
- `web/src/components/SourceLines.vue` (+ test)
- `web/src/views/BuildsView.vue`
- `web/src/views/BuildDetailView.vue` (+ test)
- `web/src/views/SourceView.vue` (+ test)

Frontend (modify): `web/src/router/index.ts`; delete `web/src/views/RepoPlaceholder.vue`.

---

## Task 1: Backend — expose base build number

**Files:**
- Modify: `core/build.go`, `models/build.go`, `modules/build/service.go`
- Modify (tests): `modules/build/service_test.go`, `models/build_test.go`

- [ ] **Step 1: Add `BaseBuildNumber` to `core.Build`**

In `core/build.go`, add the field to the `Build` struct after `BaseBuildID`:
```go
	BaseBuildID    uint          `json:"baseBuildID"`
	BaseBuildNumber int          `json:"baseBuildNumber"`
```

- [ ] **Step 2: Add the column to `models.Build` and the converters**

In `models/build.go`:
(a) `Build` struct — add after `BaseBuildID uint`:
```go
	BaseBuildID     uint
	BaseBuildNumber int
```
(b) `ToCoreBuild` — add to the returned `&core.Build{...}`:
```go
		BaseBuildID:     m.BaseBuildID,
		BaseBuildNumber: m.BaseBuildNumber,
```
(c) `copyBuildToModel` — add:
```go
	dst.BaseBuildID = src.BaseBuildID
	dst.BaseBuildNumber = src.BaseBuildNumber
```

- [ ] **Step 3: Set it in finalize**

In `modules/build/service.go` `Finalize`, where the base build is applied:
```go
	if base, err := s.baseBuild(ctx, repo, build); err == nil && base != nil {
		build.CoverageChange = build.Coverage - base.Coverage
		build.BaseBuildID = base.ID
		build.BaseBuildNumber = base.Number
	}
```

- [ ] **Step 4: Extend the finalize test to assert it**

In `modules/build/service_test.go`, in `TestFinalizePushBuildComputesDelta`, give the base build a number and assert it propagates. Change the `LatestOnBranch` return to include `Number`, and add an assertion in the `Update` `DoAndReturn`:
```go
	builds.EXPECT().LatestOnBranch(uint(1), "master", uint(5)).Return(
		&core.Build{ID: 1, Number: 1, Coverage: 0.25}, nil,
	)
	builds.EXPECT().Update(gomock.Any()).DoAndReturn(func(b *core.Build) error {
		if b.Status != core.BuildDone {
			t.Fatal("status should be done")
		}
		if b.Coverage != 0.5 {
			t.Fatalf("coverage = %v, want 0.5", b.Coverage)
		}
		if b.CoverageChange != 0.25 {
			t.Fatalf("coverageChange = %v, want 0.25", b.CoverageChange)
		}
		if b.BaseBuildID != 1 {
			t.Fatalf("baseBuildID = %d, want 1", b.BaseBuildID)
		}
		if b.BaseBuildNumber != 1 {
			t.Fatalf("baseBuildNumber = %d, want 1", b.BaseBuildNumber)
		}
		return nil
	})
```

- [ ] **Step 5: Assert persistence in the store round-trip test**

In `models/build_test.go` `TestBuildStoreUpdateAndLatestOnBranch`, after setting `first.Status`/`first.Coverage`, also set `first.BaseBuildNumber = 7` before `store.Update(first)`, and after `FindByNumber` assert it round-trips:
```go
	first.Status = core.BuildDone
	first.Coverage = 0.8
	first.BaseBuildNumber = 7
	first.SourceFiles = []*core.SourceFile{{Name: "a.go", Coverage: []*int{intPtr(1)}}}
	if err := store.Update(first); err != nil {
		t.Fatal(err)
	}
	got, err := store.FindByNumber(1, first.Number)
	if err != nil {
		t.Fatal(err)
	}
	if got.BaseBuildNumber != 7 {
		t.Fatalf("baseBuildNumber = %d, want 7", got.BaseBuildNumber)
	}
```
(Insert the `BaseBuildNumber` assertion alongside the existing status/coverage/source-file checks; keep the rest of the test intact.)

- [ ] **Step 6: Verify**

Run: `go build ./core/... ./models/... ./modules/...` → success.
Run: `go test ./modules/build/ -run TestFinalize -count=1` → PASS (the finalize tests run without CGO).
Run: `go vet ./models/...` → clean (the models store test needs CGO to run; vet typechecks it). Do NOT run `go test ./models/...` (CGO sqlite stub).

- [ ] **Step 7: Commit**

```bash
git add core/build.go models/build.go modules/build/service.go modules/build/service_test.go models/build_test.go
git commit -m "build: expose base build number for per-file deltas"
```

---

## Task 2: Frontend build types

**Files:**
- Create: `web/src/types/build.ts`

- [ ] **Step 1: Write `web/src/types/build.ts`**

```ts
export interface SourceFile {
  name: string
  source_digest: string
  coverage: (number | null)[]
}

export interface Job {
  id: number
  serviceJobID: string
  coverage: number
}

export type BuildStatus = 'processing' | 'done' | 'errored'

export interface Build {
  number: number
  serviceName: string
  serviceNumber: string
  commit: string
  branch: string
  pullRequest: number
  status: BuildStatus
  parallel: boolean
  coverage: number
  coverageChange: number
  baseBuildID: number
  baseBuildNumber: number
  commitMessage: string
  authorName: string
  authorEmail: string
  sourceFiles?: SourceFile[]
  jobs?: Job[]
  createdAt: string
  finishedAt: string
}
```

- [ ] **Step 2: Type-check**

Run: `cd web && npx vue-tsc --noEmit` → no errors.

- [ ] **Step 3: Commit**

```bash
cd .. && git add web/src/types/build.ts && git commit -m "web: build TS types"
```

---

## Task 3: Coverage utilities

**Files:**
- Create: `web/src/lib/coverage.ts`, `web/src/lib/__tests__/coverage.spec.ts`

- [ ] **Step 1: Write the failing test `web/src/lib/__tests__/coverage.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { fileCoverage, lineState, formatPercent, coverageColor } from '@/lib/coverage'
import type { SourceFile } from '@/types/build'

const f = (coverage: (number | null)[]): SourceFile => ({ name: 'x', source_digest: '', coverage })

describe('coverage utils', () => {
  it('fileCoverage counts relevant and covered', () => {
    expect(fileCoverage(f([1, null, 0, 2]))).toEqual({ covered: 2, relevant: 3, ratio: 2 / 3 })
  })
  it('fileCoverage is 0 when no relevant lines', () => {
    expect(fileCoverage(f([null, null]))).toEqual({ covered: 0, relevant: 0, ratio: 0 })
  })
  it('lineState maps hits/miss/none', () => {
    expect(lineState(3)).toBe('hit')
    expect(lineState(0)).toBe('miss')
    expect(lineState(null)).toBe('none')
  })
  it('formatPercent renders one decimal', () => {
    expect(formatPercent(0.824)).toBe('82.4%')
    expect(formatPercent(0)).toBe('0.0%')
  })
  it('coverageColor thresholds', () => {
    expect(coverageColor(0.9)).toBe('success')
    expect(coverageColor(0.6)).toBe('warning')
    expect(coverageColor(0.2)).toBe('error')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx vitest run src/lib/__tests__/coverage.spec.ts` → FAIL (cannot resolve `@/lib/coverage`).

- [ ] **Step 3: Implement `web/src/lib/coverage.ts`**

```ts
import type { SourceFile } from '@/types/build'

export interface FileCoverage {
  covered: number
  relevant: number
  ratio: number
}

// fileCoverage computes covered/relevant statement counts and their ratio.
// relevant = non-null entries; covered = entries > 0; ratio 0 when none relevant.
export function fileCoverage(file: SourceFile): FileCoverage {
  let covered = 0
  let relevant = 0
  for (const hit of file.coverage) {
    if (hit === null) continue
    relevant++
    if (hit > 0) covered++
  }
  return { covered, relevant, ratio: relevant === 0 ? 0 : covered / relevant }
}

export type LineState = 'hit' | 'miss' | 'none'

export function lineState(hits: number | null): LineState {
  if (hits === null) return 'none'
  return hits > 0 ? 'hit' : 'miss'
}

export function formatPercent(ratio: number): string {
  return `${(ratio * 100).toFixed(1)}%`
}

// coverageColor maps a ratio to a Vuetify theme color name.
export function coverageColor(ratio: number): 'success' | 'warning' | 'error' {
  if (ratio >= 0.8) return 'success'
  if (ratio >= 0.5) return 'warning'
  return 'error'
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd web && npx vitest run src/lib/__tests__/coverage.spec.ts` → PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/lib/coverage.ts web/src/lib/__tests__/coverage.spec.ts && git commit -m "web: coverage utilities"
```

---

## Task 4: Highlight utility

**Files:**
- Create: `web/src/lib/highlight.ts`, `web/src/lib/__tests__/highlight.spec.ts`

- [ ] **Step 1: Write the failing test `web/src/lib/__tests__/highlight.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { languageFor, highlightLine } from '@/lib/highlight'

describe('highlight utils', () => {
  it('languageFor maps known extensions', () => {
    expect(languageFor('a/b/main.go')).toBe('go')
    expect(languageFor('x.ts')).toBe('typescript')
    expect(languageFor('y.py')).toBe('python')
    expect(languageFor('noext')).toBeUndefined()
    expect(languageFor('weird.xyz')).toBeUndefined()
  })
  it('highlightLine escapes HTML for unknown language', () => {
    expect(highlightLine('a < b && c', undefined)).toBe('a &lt; b &amp;&amp; c')
  })
  it('highlightLine returns markup for a known language', () => {
    const html = highlightLine('const x = 1', 'typescript')
    expect(html).toContain('hljs-keyword')
    expect(html).toContain('x')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx vitest run src/lib/__tests__/highlight.spec.ts` → FAIL.

- [ ] **Step 3: Implement `web/src/lib/highlight.ts`**

```ts
import hljs from 'highlight.js/lib/common'
import 'highlight.js/styles/github.css'

const EXT_LANG: Record<string, string> = {
  go: 'go', js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
  ts: 'typescript', tsx: 'typescript', py: 'python', rb: 'ruby', java: 'java',
  c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', hpp: 'cpp', cs: 'csharp', php: 'php',
  rs: 'rust', kt: 'kotlin', swift: 'swift', scala: 'scala', sh: 'bash', bash: 'bash',
  json: 'json', yaml: 'yaml', yml: 'yaml', xml: 'xml', html: 'xml', css: 'css',
  scss: 'scss', sql: 'sql', md: 'markdown'
}

// languageFor returns the highlight.js language id for a file path, or undefined.
export function languageFor(path: string): string | undefined {
  const dot = path.lastIndexOf('.')
  if (dot < 0) return undefined
  const ext = path.slice(dot + 1).toLowerCase()
  const lang = EXT_LANG[ext]
  if (!lang) return undefined
  return hljs.getLanguage(lang) ? lang : undefined
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

// highlightLine returns HTML for a single source line. Highlights with the given
// language (balanced per-line markup); falls back to HTML-escaped plain text.
export function highlightLine(line: string, language: string | undefined): string {
  if (!language) return escapeHtml(line)
  try {
    return hljs.highlight(line, { language, ignoreIllegal: true }).value
  } catch {
    return escapeHtml(line)
  }
}
```
Note: per-line highlighting yields balanced HTML for the line table (no cross-line `<span>` breakage); multi-line strings/comments restart per line — an accepted trade-off for the coverage table.

- [ ] **Step 4: Run to verify it passes**

Run: `cd web && npx vitest run src/lib/__tests__/highlight.spec.ts` → PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/lib/highlight.ts web/src/lib/__tests__/highlight.spec.ts && git commit -m "web: source highlight utility"
```

---

## Task 5: Build store

**Files:**
- Create: `web/src/stores/build.ts`, `web/src/stores/__tests__/build.spec.ts`

- [ ] **Step 1: Write the failing test `web/src/stores/__tests__/build.spec.ts`**

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useBuildStore } from '@/stores/build'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const repoPath = '/api/v1/repos/github/o/r'

describe('build store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('fetchList populates list', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ number: 1 }, { number: 2 }] })
    const s = useBuildStore()
    await s.fetchList(repoPath)
    expect(s.list).toHaveLength(2)
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/builds`)
  })

  it('fetchBuild loads current and the base build when baseBuildNumber>0', async () => {
    ;(http.get as any)
      .mockResolvedValueOnce({ data: { number: 5, baseBuildNumber: 3, sourceFiles: [] } })
      .mockResolvedValueOnce({ data: { number: 3, baseBuildNumber: 0, sourceFiles: [] } })
    const s = useBuildStore()
    await s.fetchBuild(repoPath, 5)
    expect(s.current?.number).toBe(5)
    expect(s.base?.number).toBe(3)
    expect(http.get).toHaveBeenNthCalledWith(1, `${repoPath}/builds/5`)
    expect(http.get).toHaveBeenNthCalledWith(2, `${repoPath}/builds/3`)
  })

  it('fetchBuild leaves base null when baseBuildNumber is 0', async () => {
    ;(http.get as any).mockResolvedValue({ data: { number: 5, baseBuildNumber: 0 } })
    const s = useBuildStore()
    await s.fetchBuild(repoPath, 5)
    expect(s.base).toBeNull()
    expect(http.get).toHaveBeenCalledTimes(1)
  })

  it('fetchSource returns text', async () => {
    ;(http.get as any).mockResolvedValue({ data: 'line1\nline2' })
    const s = useBuildStore()
    const text = await s.fetchSource(repoPath, 'a/b.go', 'sha1')
    expect(text).toBe('line1\nline2')
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/content/a/b.go`, { params: { gitref: 'sha1' } })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx vitest run src/stores/__tests__/build.spec.ts` → FAIL.

- [ ] **Step 3: Implement `web/src/stores/build.ts`**

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Build } from '@/types/build'

export const useBuildStore = defineStore('build', () => {
  const list = ref<Build[]>([])
  const current = ref<Build | null>(null)
  const base = ref<Build | null>(null)

  async function fetchList(repoPath: string) {
    const { data } = await http.get<Build[]>(`${repoPath}/builds`)
    list.value = data ?? []
  }

  async function fetchBuild(repoPath: string, number: number) {
    const { data } = await http.get<Build>(`${repoPath}/builds/${number}`)
    current.value = data
    base.value = null
    if (data && data.baseBuildNumber > 0) {
      const res = await http.get<Build>(`${repoPath}/builds/${data.baseBuildNumber}`)
      base.value = res.data
    }
  }

  async function fetchSource(repoPath: string, path: string, commit: string): Promise<string> {
    const { data } = await http.get<string>(`${repoPath}/content/${path}`, { params: { gitref: commit } })
    return typeof data === 'string' ? data : String(data)
  }

  return { list, current, base, fetchList, fetchBuild, fetchSource }
})
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd web && npx vitest run src/stores/__tests__/build.spec.ts` → PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/stores/build.ts web/src/stores/__tests__/build.spec.ts && git commit -m "web: build store"
```

---

## Task 6: Builds list (BuildList + BuildsView)

**Files:**
- Create: `web/src/components/BuildList.vue`, `web/src/views/BuildsView.vue`, `web/src/components/__tests__/BuildList.spec.ts`

- [ ] **Step 1: Create `web/src/components/BuildList.vue`**

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Build } from '@/types/build'
import { formatPercent, coverageColor } from '@/lib/coverage'

const props = defineProps<{ builds: Build[]; repoRoute: string }>()
const router = useRouter()
const branch = ref<string>('')

const branches = computed(() => {
  const set = new Set(props.builds.map((b) => b.branch).filter(Boolean))
  return Array.from(set).sort()
})

const filtered = computed(() =>
  branch.value ? props.builds.filter((b) => b.branch === branch.value) : props.builds
)

function refLabel(b: Build): string {
  return b.pullRequest > 0 ? `PR #${b.pullRequest} → ${b.branch}` : b.branch
}

function shortSha(sha: string): string {
  return sha ? sha.slice(0, 8) : ''
}

function open(b: Build) {
  router.push(`${props.repoRoute}/builds/${b.number}`)
}
</script>

<template>
  <div>
    <v-select
      v-model="branch"
      :items="branches"
      label="Branch"
      density="compact"
      clearable
      hide-details
      style="max-width: 260px"
      class="mb-3"
    />
    <v-table v-if="filtered.length" hover>
      <thead>
        <tr>
          <th>Build</th><th>Ref</th><th>Commit</th><th>Coverage</th><th>Δ</th><th>Status</th><th>When</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="b in filtered" :key="b.number" style="cursor: pointer" @click="open(b)">
          <td>#{{ b.number }}</td>
          <td>{{ refLabel(b) }}</td>
          <td>
            <code>{{ shortSha(b.commit) }}</code>
            <span class="text-medium-emphasis ml-2">{{ b.commitMessage.split('\n')[0] }}</span>
          </td>
          <td>
            <v-chip :color="coverageColor(b.coverage)" size="small" label>{{ formatPercent(b.coverage) }}</v-chip>
          </td>
          <td>
            <span v-if="b.coverageChange > 0" class="text-success">▲ {{ formatPercent(b.coverageChange) }}</span>
            <span v-else-if="b.coverageChange < 0" class="text-error">▼ {{ formatPercent(Math.abs(b.coverageChange)) }}</span>
            <span v-else>—</span>
          </td>
          <td>{{ b.status }}</td>
          <td>{{ new Date(b.createdAt).toLocaleString() }}</td>
        </tr>
      </tbody>
    </v-table>
    <v-alert v-else type="info" variant="tonal">No builds yet.</v-alert>
  </div>
</template>
```

- [ ] **Step 2: Create `web/src/views/BuildsView.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import http, { errorMessage } from '@/plugins/http'
import { useBuildStore } from '@/stores/build'
import BuildList from '@/components/BuildList.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const repoRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}`)

async function load() {
  loading.value = true
  error.value = ''
  try {
    await store.fetchList(repoPath.value)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h2 class="text-h5 mb-4">{{ namespace }}/{{ name }}</h2>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-alert v-if="error" type="warning" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <BuildList v-else :builds="store.list" :repo-route="repoRoute" />
  </v-container>
</template>
```

- [ ] **Step 3: Write `web/src/components/__tests__/BuildList.spec.ts`**

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import BuildList from '@/components/BuildList.vue'
import type { Build } from '@/types/build'

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const build = (over: Partial<Build>): Build => ({
  number: 1, serviceName: '', serviceNumber: '', commit: 'abcdef1234', branch: 'main', pullRequest: 0,
  status: 'done', parallel: false, coverage: 0.82, coverageChange: 0.01, baseBuildID: 0, baseBuildNumber: 0,
  commitMessage: 'msg', authorName: 'a', authorEmail: 'e', createdAt: '2020-01-01T00:00:00Z', finishedAt: '', ...over
})

function mountList(builds: Build[]) {
  return mount(BuildList, { props: { builds, repoRoute: '/report/github/o/r' }, global: { plugins: [vuetify, router] } })
}

describe('BuildList', () => {
  beforeEach(() => {})

  it('renders rows and PR label', () => {
    const w = mountList([build({ number: 7, pullRequest: 0, branch: 'main' }), build({ number: 8, pullRequest: 45, branch: 'main' })])
    expect(w.text()).toContain('#7')
    expect(w.text()).toContain('#8')
    expect(w.text()).toContain('PR #45 → main')
  })

  it('shows empty state', () => {
    const w = mountList([])
    expect(w.text()).toContain('No builds yet')
  })
})
```

- [ ] **Step 4: Run tests**

Run: `cd web && npx vitest run src/components/__tests__/BuildList.spec.ts` → PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/components/BuildList.vue web/src/views/BuildsView.vue web/src/components/__tests__/BuildList.spec.ts
git commit -m "web: builds list view"
```

---

## Task 7: Build detail (BuildSummary + FileCoverageTable + BuildDetailView)

**Files:**
- Create: `web/src/components/BuildSummary.vue`, `web/src/components/FileCoverageTable.vue`, `web/src/views/BuildDetailView.vue`, `web/src/components/__tests__/FileCoverageTable.spec.ts`, `web/src/views/__tests__/BuildDetailView.spec.ts`

- [ ] **Step 1: Create `web/src/components/BuildSummary.vue`**

```vue
<script setup lang="ts">
import type { Build } from '@/types/build'
import { formatPercent, coverageColor } from '@/lib/coverage'

defineProps<{ build: Build }>()
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="d-flex align-center ga-4">
      <v-progress-circular :model-value="build.coverage * 100" :color="coverageColor(build.coverage)" size="72" width="8">
        {{ formatPercent(build.coverage) }}
      </v-progress-circular>
      <div>
        <div class="text-h6">Build #{{ build.number }} · {{ build.branch }}</div>
        <div class="text-body-2">
          <code>{{ build.commit.slice(0, 8) }}</code> {{ build.commitMessage.split('\n')[0] }}
        </div>
        <div class="text-medium-emphasis text-body-2">
          {{ build.authorName }} · {{ new Date(build.createdAt).toLocaleString() }} ·
          {{ build.jobs?.length || 0 }} job(s) · {{ build.status }}
          <span v-if="build.baseBuildNumber > 0">
            ·
            <span :class="build.coverageChange >= 0 ? 'text-success' : 'text-error'">
              {{ build.coverageChange >= 0 ? '+' : '' }}{{ formatPercent(build.coverageChange) }}
            </span>
            vs #{{ build.baseBuildNumber }}
          </span>
        </div>
      </div>
    </div>
  </v-card>
</template>
```

- [ ] **Step 2: Create `web/src/components/FileCoverageTable.vue`**

```vue
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
```

- [ ] **Step 3: Create `web/src/views/BuildDetailView.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { useBuildStore } from '@/stores/build'
import BuildSummary from '@/components/BuildSummary.vue'
import FileCoverageTable from '@/components/FileCoverageTable.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const number = computed(() => Number(route.params.number))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const buildRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}/builds/${number.value}`)

async function load() {
  loading.value = true
  error.value = ''
  try {
    await store.fetchBuild(repoPath.value, number.value)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(number, load)
</script>

<template>
  <v-container class="py-6">
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-alert v-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <template v-else-if="store.current">
      <BuildSummary :build="store.current" />
      <FileCoverageTable :build="store.current" :base="store.base" :build-route="buildRoute" />
    </template>
  </v-container>
</template>
```

- [ ] **Step 4: Write `web/src/components/__tests__/FileCoverageTable.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import FileCoverageTable from '@/components/FileCoverageTable.vue'
import type { Build } from '@/types/build'

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const p = (v: number) => v
const build = (files: any[]): Build => ({
  number: 5, serviceName: '', serviceNumber: '', commit: 'c', branch: 'main', pullRequest: 0, status: 'done',
  parallel: false, coverage: 0.5, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '',
  authorName: '', authorEmail: '', sourceFiles: files, jobs: [], createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
})

describe('FileCoverageTable', () => {
  it('computes per-file coverage and delta vs base', () => {
    const current = build([{ name: 'a.go', source_digest: '', coverage: [p(1), p(1), p(0), p(0)] }]) // 50%
    const base = build([{ name: 'a.go', source_digest: '', coverage: [p(1), p(0), p(0), p(0)] }]) // 25%
    const w = mount(FileCoverageTable, {
      props: { build: current, base, buildRoute: '/report/github/o/r/builds/5' },
      global: { plugins: [vuetify, router] }
    })
    expect(w.text()).toContain('a.go')
    expect(w.text()).toContain('50.0%')
    expect(w.text()).toContain('+25.0%') // delta 50% - 25%
  })

  it('shows dash delta when no base', () => {
    const current = build([{ name: 'a.go', source_digest: '', coverage: [p(1), p(0)] }])
    const w = mount(FileCoverageTable, {
      props: { build: current, base: null, buildRoute: '/r' },
      global: { plugins: [vuetify, router] }
    })
    expect(w.text()).toContain('a.go')
    expect(w.text()).toContain('—')
  })
})
```

- [ ] **Step 5: Write `web/src/views/__tests__/BuildDetailView.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import BuildDetailView from '@/views/BuildDetailView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components, directives })

function makeRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/report/:scm/:namespace/:name/builds/:number', component: BuildDetailView }, { path: '/:p(.*)*', component: { template: '<div/>' } }]
  })
  return router
}

describe('BuildDetailView', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loads and renders the build summary', async () => {
    ;(http.get as any).mockResolvedValue({ data: {
      number: 5, branch: 'main', commit: 'abcdef1234', commitMessage: 'hello', coverage: 0.82,
      coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, authorName: 'a', status: 'done',
      sourceFiles: [{ name: 'a.go', source_digest: '', coverage: [1, 0] }], jobs: [], createdAt: '2020-01-01T00:00:00Z'
    } })
    const router = makeRouter()
    router.push('/report/github/o/r/builds/5')
    await router.isReady()
    const w = mount(BuildDetailView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Build #5')
    expect(w.text()).toContain('a.go')
  })
})
```

- [ ] **Step 6: Run tests**

Run: `cd web && npx vitest run src/components/__tests__/FileCoverageTable.spec.ts src/views/__tests__/BuildDetailView.spec.ts` → PASS.

- [ ] **Step 7: Commit**

```bash
cd .. && git add web/src/components/BuildSummary.vue web/src/components/FileCoverageTable.vue web/src/views/BuildDetailView.vue web/src/components/__tests__/FileCoverageTable.spec.ts web/src/views/__tests__/BuildDetailView.spec.ts
git commit -m "web: build detail view with per-file delta"
```

---

## Task 8: Source view (SourceLines + SourceView)

**Files:**
- Create: `web/src/components/SourceLines.vue`, `web/src/views/SourceView.vue`, `web/src/components/__tests__/SourceLines.spec.ts`, `web/src/views/__tests__/SourceView.spec.ts`
- Modify: `web/src/assets/styles/variables.scss` (coverage line colors, if not already usable)

- [ ] **Step 1: Ensure coverage line colors exist in `web/src/assets/styles/variables.scss`**

Append (idempotent — only add if absent):
```scss
.statement-hit { background: rgba(46, 125, 50, 0.18); }
.statement-miss { background: rgba(198, 40, 40, 0.20); }
```

- [ ] **Step 2: Create `web/src/components/SourceLines.vue`**

```vue
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
```

- [ ] **Step 3: Create `web/src/views/SourceView.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import { errorMessage } from '@/plugins/http'
import { useBuildStore } from '@/stores/build'
import { basePath } from '@/lib/base'
import SourceLines from '@/components/SourceLines.vue'

const route = useRoute()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')
const needLogin = ref(false)
const source = ref('')

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const number = computed(() => Number(route.params.number))
const path = computed(() => Array.isArray(route.params.path) ? route.params.path.join('/') : String(route.params.path))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const loginUrl = computed(() => `${basePath()}/login?redirect=${encodeURIComponent(route.fullPath)}`)

const coverage = computed<(number | null)[]>(() => {
  const f = store.current?.sourceFiles?.find((s) => s.name === path.value)
  return f?.coverage ?? []
})

async function load() {
  loading.value = true
  error.value = ''
  needLogin.value = false
  try {
    if (!store.current || store.current.number !== number.value) {
      await store.fetchBuild(repoPath.value, number.value)
    }
    const commit = store.current?.commit ?? ''
    source.value = await store.fetchSource(repoPath.value, path.value, commit)
  } catch (e) {
    if (axios.isAxiosError(e) && e.response?.status === 401) {
      needLogin.value = true
    } else {
      error.value = errorMessage(e)
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h3 class="text-subtitle-1 mb-3"><code>{{ path }}</code> · build #{{ number }}</h3>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-card v-else-if="needLogin" class="pa-6 text-center">
      <p class="mb-4">Sign in to view source for this repository.</p>
      <v-btn color="primary" :href="loginUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <SourceLines v-else :source="source" :coverage="coverage" :path="path" />
  </v-container>
</template>
```

- [ ] **Step 4: Write `web/src/components/__tests__/SourceLines.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import SourceLines from '@/components/SourceLines.vue'

const vuetify = createVuetify({ components })

describe('SourceLines', () => {
  it('applies hit/miss classes and hit counts', () => {
    const w = mount(SourceLines, {
      props: { source: 'a\nb\nc', coverage: [3, 0, null], path: 'x.txt' },
      global: { plugins: [vuetify] }
    })
    const rows = w.findAll('tr')
    expect(rows).toHaveLength(3)
    expect(rows[0].classes()).toContain('statement-hit')
    expect(rows[0].text()).toContain('3×')
    expect(rows[1].classes()).toContain('statement-miss')
    expect(rows[2].classes()).not.toContain('statement-hit')
    expect(rows[2].classes()).not.toContain('statement-miss')
  })
})
```

- [ ] **Step 5: Write `web/src/views/__tests__/SourceView.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import SourceView from '@/views/SourceView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
// SourceView calls axios.isAxiosError to detect the 401; stub it (vi.mock is hoisted).
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components })

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/report/:scm/:namespace/:name/builds/:number/source/:path(.*)*', component: SourceView },
      { path: '/:p(.*)*', component: { template: '<div/>' } }
    ]
  })
}

describe('SourceView', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows sign-in prompt on 401 from content', async () => {
    ;(http.get as any).mockImplementation((url: string) => {
      if (url.includes('/builds/')) return Promise.resolve({ data: { number: 5, commit: 'c', baseBuildNumber: 0, sourceFiles: [{ name: 'a.go', source_digest: '', coverage: [1] }] } })
      return Promise.reject({ isAxiosError: true, response: { status: 401 } })
    })
    const router = makeRouter()
    router.push('/report/github/o/r/builds/5/source/a.go')
    await router.isReady()
    const w = mount(SourceView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Sign in to view source')
  })
})
```
Note: if mocking `axios.isAxiosError` at the top of the file is cleaner, hoist the `vi.mock('axios', ...)` to module top (above imports) — Vitest hoists `vi.mock`. Adjust if the inline mock does not take effect; the assertion (sign-in prompt visible) is the contract.

- [ ] **Step 6: Run tests**

Run: `cd web && npx vitest run src/components/__tests__/SourceLines.spec.ts src/views/__tests__/SourceView.spec.ts` → PASS.

- [ ] **Step 7: Commit**

```bash
cd .. && git add web/src/components/SourceLines.vue web/src/views/SourceView.vue web/src/components/__tests__/SourceLines.spec.ts web/src/views/__tests__/SourceView.spec.ts web/src/assets/styles/variables.scss
git commit -m "web: per-line source view"
```

---

## Task 9: Wire routes and final verification

**Files:**
- Modify: `web/src/router/index.ts`
- Delete: `web/src/views/RepoPlaceholder.vue`

- [ ] **Step 1: Replace the placeholder route with the three B2 routes**

In `web/src/router/index.ts`:
(a) Replace the `RepoPlaceholder` import with the three views:
```ts
import BuildsView from '@/views/BuildsView.vue'
import BuildDetailView from '@/views/BuildDetailView.vue'
import SourceView from '@/views/SourceView.vue'
```
(remove `import RepoPlaceholder from '@/views/RepoPlaceholder.vue'`).
(b) Replace the single `repo` route object with:
```ts
    { path: '/report/:scm/:namespace/:name', name: 'repo', component: BuildsView },
    { path: '/report/:scm/:namespace/:name/builds/:number', name: 'build', component: BuildDetailView },
    { path: '/report/:scm/:namespace/:name/builds/:number/source/:path(.*)*', name: 'source', component: SourceView },
```
(Note: these are public — no `meta: { requiresAuth: true }`. The API enforces per-repo access; private repos surface 401 inline.)

- [ ] **Step 2: Delete the placeholder**

```bash
git rm web/src/views/RepoPlaceholder.vue
```

- [ ] **Step 3: Full frontend verification**

```bash
cd web
npx vitest run
npm run lint
npm run build
```
Expected: all tests pass (B1's + B2's new specs); lint exit 0; `vue-tsc` + `vite build` succeed.

- [ ] **Step 4: Backend verification**

Run (repo root): `go build ./core/... ./models/... ./modules/... ./routers/...` → success.
Run: `go test ./modules/build/ -count=1` → PASS.

- [ ] **Step 5: Manual smoke test (optional, recommended)**

```bash
# terminal 1: backend
go run ./cmd/server
# terminal 2: frontend
cd web && npm run dev
# Activate a repo and upload a build via a coveralls reporter, then:
#  /report/<scm>/<ns>/<name>        → builds list
#  click a build                    → summary + file table (+ per-file Δ if a base build exists)
#  click a file                     → per-line source (sign-in prompt if not logged in)
```

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/router/index.ts
git commit -m "web: wire build routes and remove placeholder"
```

---

## Self-Review notes (spec coverage)

- **B2.1 routes/views** → Task 9 (routes), Tasks 6/7/8 (views), public + source-auth handled in Task 8.
- **B2.2 types/store/utils** → Task 2 (types), Task 3 (coverage), Task 5 (store); highlight util Task 4.
- **B2.3 builds list** → Task 6 (BuildList branch filter, PR label, columns).
- **B2.4 build detail + per-file delta** → Task 7 (BuildSummary, FileCoverageTable using base build).
- **B2.5 source view (hit count + coloring, 401 prompt)** → Task 8.
- **B2.7 backend base build number** → Task 1.
- **B2.8 testing** → tests in Tasks 1,3,4,5,6,7,8.

Deferred per spec: folder tree, branch/PR views, parallel-jobs/flags UI, virtualization (B3).
