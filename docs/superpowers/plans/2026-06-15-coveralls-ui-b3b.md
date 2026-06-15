# Coveralls UI B3b — Browsing Polish & Flags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a folder/tree file view, a branches overview, a parallel-jobs panel with flag support (incl. backend `flag_name` storage), and a PR changed-files panel.

**Architecture:** Frontend Vue 3 components + a pure tree util + build-store/types additions, plus two small backend additions (persist `Job.Flag`; a PR changed-files endpoint).

**Tech Stack:** Vue 3.5/Vuetify 3/Pinia/Vite/Vitest; Go 1.14. Node 24/npm available.

**Conventions (CLAUDE.md):** short subsystem-prefixed commits (`web:`/`build:`/`repo:`), no metadata. Frontend cmds from `web/`; Go from repo root. Backend builds with explicit paths: `go build ./core/... ./models/... ./modules/... ./routers/...`. `go test ./routers/api/build/ ./modules/build/` run; `models` tests need CGO (vet only). Never commit `web/dist`, `web/node_modules`, `web/coverage`, `web/web_gen.go`.

---

## Task 1: Backend — persist job flags

**Files:** `core/build.go`, `models/build.go`, `routers/api/build/build.go`; tests `models/build_test.go`, `routers/api/build/build_test.go`.

- [ ] **Step 1: Add `Flag` to `core.Job`**

In `core/build.go`, in the `Job` struct add after `Coverage`:
```go
	Coverage         float64       `json:"coverage"`
	Flag             string        `json:"flag"`
```

- [ ] **Step 2: Persist + map `Flag` in `models/build.go`**

(a) `Job` struct: add `Flag string` after `Coverage float64`.
(b) `AddJob`: set `m.Flag = j.Flag` in the `&Job{...}` literal.
(c) `toCoreJob`: add `Flag: m.Flag,` to the returned `&core.Job{...}`.

- [ ] **Step 3: Set flag in `HandleJobs`**

In `routers/api/build/build.go`, the job literal becomes:
```go
		job := &core.Job{
			ServiceJobID:     payload.ServiceJobID,
			ServiceJobNumber: payload.ServiceJobNumber,
			SourceFiles:      payload.SourceFiles,
			Coverage:         buildmod.Coverage(payload.SourceFiles),
			Flag:             payload.FlagName,
		}
```

- [ ] **Step 4: Store round-trip test**

In `models/build_test.go` `TestBuildStoreAddJobAndJobs`, set a flag on the job and assert it round-trips. Change the job creation to include `Flag: "unit"` and after `store.Jobs` add:
```go
	if jobs[0].Flag != "unit" {
		t.Fatalf("flag = %q, want unit", jobs[0].Flag)
	}
```

- [ ] **Step 5: Handler test**

In `routers/api/build/build_test.go` `TestHandleJobsSingleFinalizes`, change the `AddJob` expectation to capture and assert the flag, and add `"flag_name":"unit"` to the request body:
```go
	builds.EXPECT().AddJob(gomock.Any(), gomock.Any()).DoAndReturn(func(b *core.Build, j *core.Job) error {
		if j.Flag != "unit" {
			t.Fatalf("flag = %q, want unit", j.Flag)
		}
		j.ID = 3
		return nil
	})
```
and the body:
```go
	body := `{"repo_token":"tok","service_number":"5","flag_name":"unit",
		"git":{"branch":"master","head":{"id":"sha"}},
		"source_files":[{"name":"a.go","coverage":[1,0]}]}`
```

- [ ] **Step 6: Verify + commit**

Run: `go build ./core/... ./models/... ./modules/... ./routers/...` → success;
`go test ./routers/api/build/ -count=1` → PASS; `go vet ./models/...` → clean.
```bash
git add core/build.go models/build.go routers/api/build/build.go models/build_test.go routers/api/build/build_test.go
git commit -m "build: persist coverage flag on jobs"
```

---

## Task 2: Backend — PR changed-files endpoint

**Files:** `routers/api/build/build.go`, `routers/api/api.go`; test `routers/api/build/build_test.go`.

- [ ] **Step 1: Add `HandleChanges` to `routers/api/build/build.go`**

```go
// HandleChanges returns the files changed in a pull request.
// @Summary List a pull request's changed files
// @Tags Build
// @Router /repos/{scm}/{namespace}/{name}/pulls/{number}/changes [get]
func HandleChanges(
	scmService core.SCMService,
	repoStore core.RepoStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := request.UserFrom(c)
		if !ok {
			c.JSON(401, []*core.FileChange{})
			return
		}
		repo, err := repoStore.Find(&core.Repo{
			NameSpace: c.Param("namespace"),
			Name:      c.Param("name"),
			SCM:       core.SCMProvider(c.Param("scm")),
		})
		if err != nil {
			c.JSON(404, []*core.FileChange{})
			return
		}
		number, err := strconv.Atoi(c.Param("number"))
		if err != nil {
			c.JSON(400, []*core.FileChange{})
			return
		}
		client, err := scmService.Client(repo.SCM)
		if err != nil {
			c.JSON(500, []*core.FileChange{})
			return
		}
		changes, err := client.PullRequests().ListChanges(c.Request.Context(), user, repo.FullName(), number)
		if err != nil {
			c.JSON(500, []*core.FileChange{})
			return
		}
		c.JSON(200, changes)
	}
}
```
(`request` and `strconv` are already imported in build.go.)

- [ ] **Step 2: Register the route (authenticated group)**

In `routers/api/api.go`, in the authenticated repo group (alongside `/content/*path`):
```go
			g.GET("/pulls/:number/changes", build.HandleChanges(r.SCMService, r.RepoStore))
```

- [ ] **Step 3: Handler test**

In `routers/api/build/build_test.go`, add a `fakePRService` (mirroring `modules/build/service_test.go` — there is no generated `PullRequestService` mock) and a test:
```go
type fakePRChanges struct{ changes []*core.FileChange }

func (f *fakePRChanges) Find(ctx context.Context, user *core.User, repo string, number int) (*core.PullRequest, error) { return nil, nil }
func (f *fakePRChanges) CreateComment(ctx context.Context, user *core.User, repo string, number int, body string) (int, error) { return 0, nil }
func (f *fakePRChanges) RemoveComment(ctx context.Context, user *core.User, repo string, number int, id int) error { return nil }
func (f *fakePRChanges) ListChanges(ctx context.Context, user *core.User, repo string, number int) ([]*core.FileChange, error) { return f.changes, nil }

func TestHandleChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)

	repos.EXPECT().Find(gomock.Any()).Return(&core.Repo{NameSpace: "o", Name: "r", SCM: core.Github}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().PullRequests().Return(&fakePRChanges{changes: []*core.FileChange{{Path: "a.go"}}})

	r := gin.New()
	r.Use(func(c *gin.Context) { request.WithUser(c, &core.User{Login: "u"}) })
	r.GET("/api/v1/repos/:scm/:namespace/:name/pulls/:number/changes", HandleChanges(scm, repos))
	req := httptest.NewRequest("GET", "/api/v1/repos/github/o/r/pulls/3/changes", nil)
	if w := serve(r, req); w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandleChangesUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	r := gin.New()
	r.GET("/api/v1/repos/:scm/:namespace/:name/pulls/:number/changes", HandleChanges(scm, repos))
	req := httptest.NewRequest("GET", "/api/v1/repos/github/o/r/pulls/3/changes", nil)
	if w := serve(r, req); w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
```
Add `"context"` and `"github.com/covergates/covergates/routers/api/request"` to the test imports if missing (`request` is needed for `WithUser`; `serve` already exists in build_test.go).

- [ ] **Step 4: Verify + commit**

Run: `go build ./...routers` paths + `go test ./routers/api/build/ -count=1` → PASS.
```bash
git add routers/api/build/build.go routers/api/api.go routers/api/build/build_test.go
git commit -m "build: pull request changed-files endpoint"
```

---

## Task 3: Frontend types + build-store fetchChanges

**Files:** `web/src/types/build.ts`, `web/src/stores/build.ts`; test `web/src/stores/__tests__/build.spec.ts`.

- [ ] **Step 1: Extend `web/src/types/build.ts`**

Add `flag: string` to `Job` (after `coverage`), and a `FileChange` interface:
```ts
export interface Job {
  id: number
  serviceJobID: string
  coverage: number
  flag: string
}

export interface FileChange {
  path: string
  added: boolean
  renamed: boolean
  deleted: boolean
}
```

- [ ] **Step 2: Add `fetchChanges` to `web/src/stores/build.ts`**

Add `import type { Build, FileChange } from '@/types/build'` (extend the existing import) and an action; include it in the return:
```ts
  async function fetchChanges(repoPath: string, number: number): Promise<FileChange[]> {
    const { data } = await http.get<FileChange[]>(`${repoPath}/pulls/${number}/changes`)
    return data ?? []
  }
```
Return: add `fetchChanges`.

- [ ] **Step 3: Test**

Append to `web/src/stores/__tests__/build.spec.ts`:
```ts
  it('fetchChanges hits the pulls changes endpoint', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ path: 'a.go', added: true, renamed: false, deleted: false }] })
    const s = useBuildStore()
    const changes = await s.fetchChanges(repoPath, 3)
    expect(changes).toHaveLength(1)
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/pulls/3/changes`)
  })
```
(The file's `beforeEach` already clears mocks.)

- [ ] **Step 4: Verify + commit**

Run: `cd web && npx vitest run src/stores/__tests__/build.spec.ts` → PASS.
```bash
cd .. && git add web/src/types/build.ts web/src/stores/build.ts web/src/stores/__tests__/build.spec.ts
git commit -m "web: job flag type and fetchChanges"
```

---

## Task 4: File tree utility

**Files:** `web/src/lib/fileTree.ts`, `web/src/lib/__tests__/fileTree.spec.ts`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from 'vitest'
import { buildTree } from '@/lib/fileTree'

describe('buildTree', () => {
  it('nests files into folders and aggregates coverage', () => {
    const root = buildTree([
      { path: 'a/b.go', covered: 1, relevant: 2 },
      { path: 'a/c.go', covered: 2, relevant: 2 },
      { path: 'd.go', covered: 0, relevant: 1 }
    ])
    // root aggregates everything: covered 3, relevant 5
    expect(root.covered).toBe(3)
    expect(root.relevant).toBe(5)
    const names = root.children.map((c) => c.name)
    expect(names).toEqual(['a', 'd.go']) // folders first, then files
    const a = root.children.find((c) => c.name === 'a')!
    expect(a.isDir).toBe(true)
    expect(a.covered).toBe(3)
    expect(a.relevant).toBe(4)
    expect(a.children.map((c) => c.name)).toEqual(['b.go', 'c.go'])
    const d = root.children.find((c) => c.name === 'd.go')!
    expect(d.isDir).toBe(false)
    expect(d.path).toBe('d.go')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx vitest run src/lib/__tests__/fileTree.spec.ts` → FAIL.

- [ ] **Step 3: Implement `web/src/lib/fileTree.ts`**

```ts
export interface FileLeaf {
  path: string
  covered: number
  relevant: number
}

export interface TreeNode {
  name: string
  path: string
  isDir: boolean
  covered: number
  relevant: number
  children: TreeNode[]
}

function newDir(name: string, path: string): TreeNode {
  return { name, path, isDir: true, covered: 0, relevant: 0, children: [] }
}

// buildTree builds a directory tree from flat file paths, aggregating
// covered/relevant counts up to every ancestor folder. Children are sorted
// folders-first, then alphabetically.
export function buildTree(files: FileLeaf[]): TreeNode {
  const root = newDir('', '')
  for (const f of files) {
    const parts = f.path.split('/')
    let node = root
    node.covered += f.covered
    node.relevant += f.relevant
    let prefix = ''
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      prefix = prefix ? `${prefix}/${part}` : part
      const last = i === parts.length - 1
      if (last) {
        node.children.push({ name: part, path: f.path, isDir: false, covered: f.covered, relevant: f.relevant, children: [] })
      } else {
        let child = node.children.find((c) => c.isDir && c.name === part)
        if (!child) {
          child = newDir(part, prefix)
          node.children.push(child)
        }
        child.covered += f.covered
        child.relevant += f.relevant
        node = child
      }
    }
  }
  sortTree(root)
  return root
}

function sortTree(node: TreeNode) {
  node.children.sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    return a.name.localeCompare(b.name)
  })
  for (const c of node.children) if (c.isDir) sortTree(c)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd web && npx vitest run src/lib/__tests__/fileTree.spec.ts` → PASS.

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/lib/fileTree.ts web/src/lib/__tests__/fileTree.spec.ts
git commit -m "web: file tree builder"
```

---

## Task 5: FileCoverageTree component

**Files:** `web/src/components/FileCoverageTree.vue`, `web/src/components/__tests__/FileCoverageTree.spec.ts`.

- [ ] **Step 1: Create `web/src/components/FileCoverageTree.vue`**

```vue
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

// Flatten the tree into rows with a depth, expanding all folders by default.
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
```

- [ ] **Step 2: Write `web/src/components/__tests__/FileCoverageTree.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import FileCoverageTree from '@/components/FileCoverageTree.vue'
import type { Build } from '@/types/build'

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const build = (files: any[]): Build => ({
  number: 5, serviceName: '', serviceNumber: '', commit: 'c', branch: 'main', pullRequest: 0, status: 'done',
  parallel: false, coverage: 0.5, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '',
  authorName: '', authorEmail: '', sourceFiles: files, jobs: [], createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
})

describe('FileCoverageTree', () => {
  it('renders folders and files', () => {
    const w = mount(FileCoverageTree, {
      props: {
        build: build([
          { name: 'a/b.go', source_digest: '', coverage: [1, 0] },
          { name: 'd.go', source_digest: '', coverage: [1] }
        ]),
        buildRoute: '/report/github/o/r/builds/5'
      },
      global: { plugins: [vuetify, router] }
    })
    expect(w.text()).toContain('a')
    expect(w.text()).toContain('b.go')
    expect(w.text()).toContain('d.go')
  })
})
```

- [ ] **Step 3: Run + commit**

Run: `cd web && npx vitest run src/components/__tests__/FileCoverageTree.spec.ts` → PASS.
```bash
cd .. && git add web/src/components/FileCoverageTree.vue web/src/components/__tests__/FileCoverageTree.spec.ts
git commit -m "web: folder coverage tree"
```

---

## Task 6: Jobs panel

**Files:** `web/src/components/JobsPanel.vue`, `web/src/components/__tests__/JobsPanel.spec.ts`.

- [ ] **Step 1: Create `web/src/components/JobsPanel.vue`**

```vue
<script setup lang="ts">
import type { Job } from '@/types/build'
import { formatPercent, coverageColor } from '@/lib/coverage'

defineProps<{ jobs: Job[] }>()
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Jobs</div>
    <v-table density="compact">
      <thead>
        <tr><th>Job</th><th>Flag</th><th>Coverage</th></tr>
      </thead>
      <tbody>
        <tr v-for="(job, i) in jobs" :key="job.id || i">
          <td>{{ job.serviceJobID || ('#' + (i + 1)) }}</td>
          <td>{{ job.flag || '—' }}</td>
          <td><v-chip :color="coverageColor(job.coverage)" size="x-small" label>{{ formatPercent(job.coverage) }}</v-chip></td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
```

- [ ] **Step 2: Write `web/src/components/__tests__/JobsPanel.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import JobsPanel from '@/components/JobsPanel.vue'

const vuetify = createVuetify({ components })

describe('JobsPanel', () => {
  it('renders job flags and coverage', () => {
    const w = mount(JobsPanel, {
      props: { jobs: [{ id: 1, serviceJobID: 'j1', coverage: 0.8, flag: 'unit' }, { id: 2, serviceJobID: 'j2', coverage: 0.5, flag: '' }] },
      global: { plugins: [vuetify] }
    })
    expect(w.text()).toContain('unit')
    expect(w.text()).toContain('—')
    expect(w.text()).toContain('80.0%')
  })
})
```

- [ ] **Step 3: Run + commit**

Run: `cd web && npx vitest run src/components/__tests__/JobsPanel.spec.ts` → PASS.
```bash
cd .. && git add web/src/components/JobsPanel.vue web/src/components/__tests__/JobsPanel.spec.ts
git commit -m "web: jobs and flags panel"
```

---

## Task 7: PR changed-files panel

**Files:** `web/src/components/ChangedFilesPanel.vue`, `web/src/components/__tests__/ChangedFilesPanel.spec.ts`.

- [ ] **Step 1: Create `web/src/components/ChangedFilesPanel.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import { fileCoverage, formatPercent, coverageColor } from '@/lib/coverage'
import type { Build, FileChange } from '@/types/build'

const props = defineProps<{ build: Build; repoPath: string; buildRoute: string; loginRedirect: string }>()
const router = useRouter()
const store = useBuildStore()

const loading = ref(false)
const needLogin = ref(false)
const error = ref('')
const changes = ref<FileChange[]>([])

const coverageByPath = computed(() => {
  const m = new Map<string, number | null>()
  for (const f of props.build.sourceFiles ?? []) {
    const c = fileCoverage(f)
    m.set(f.name, c.relevant === 0 ? null : c.ratio)
  }
  return m
})

async function load() {
  loading.value = true
  needLogin.value = false
  error.value = ''
  try {
    changes.value = await store.fetchChanges(props.repoPath, props.build.number)
  } catch (e) {
    if (is401(e)) needLogin.value = true
    else error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function ratioOf(path: string): number | null | undefined {
  return coverageByPath.value.get(path)
}
function openSource(path: string) {
  router.push(`${props.buildRoute}/source/${path}`)
}

onMounted(load)
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Changed files (PR #{{ build.pullRequest }})</div>
    <v-progress-linear v-if="loading" indeterminate color="primary" />
    <p v-else-if="needLogin" class="text-body-2">
      <a :href="loginUrl(loginRedirect)">Sign in</a> to view changed files.
    </p>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <v-table v-else density="compact">
      <tbody>
        <tr v-for="ch in changes" :key="ch.path" :style="{ cursor: ch.deleted ? 'default' : 'pointer' }" @click="!ch.deleted && openSource(ch.path)">
          <td :class="{ 'text-decoration-line-through': ch.deleted }"><code>{{ ch.path }}</code></td>
          <td>
            <span v-if="ch.deleted" class="text-medium-emphasis">deleted</span>
            <v-chip v-else-if="ratioOf(ch.path) != null" :color="coverageColor(ratioOf(ch.path) as number)" size="x-small" label>
              {{ formatPercent(ratioOf(ch.path) as number) }}
            </v-chip>
            <span v-else class="text-medium-emphasis">no coverage data</span>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
```

- [ ] **Step 2: Write `web/src/components/__tests__/ChangedFilesPanel.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import ChangedFilesPanel from '@/components/ChangedFilesPanel.vue'
import type { Build } from '@/types/build'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }] })

const build: Build = {
  number: 5, serviceName: '', serviceNumber: '', commit: 'c', branch: 'feature', pullRequest: 7, status: 'done',
  parallel: false, coverage: 0.5, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '',
  authorName: '', authorEmail: '', sourceFiles: [{ name: 'a.go', source_digest: '', coverage: [1, 0] }], jobs: [],
  createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
}

function mountPanel() {
  return mount(ChangedFilesPanel, {
    props: { build, repoPath: '/api/v1/repos/github/o/r', buildRoute: '/report/github/o/r/builds/5', loginRedirect: '/x' },
    global: { plugins: [vuetify, router] }
  })
}

describe('ChangedFilesPanel', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('lists changed files with coverage', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ path: 'a.go', added: false, renamed: false, deleted: false }] })
    const w = mountPanel()
    await flushPromises()
    expect(w.text()).toContain('a.go')
    expect(w.text()).toContain('50.0%')
  })

  it('shows sign-in on 401', async () => {
    ;(http.get as any).mockRejectedValue({ isAxiosError: true, response: { status: 401 } })
    const w = mountPanel()
    await flushPromises()
    expect(w.text()).toContain('Sign in')
  })
})
```

- [ ] **Step 3: Run + commit**

Run: `cd web && npx vitest run src/components/__tests__/ChangedFilesPanel.spec.ts` → PASS.
```bash
cd .. && git add web/src/components/ChangedFilesPanel.vue web/src/components/__tests__/ChangedFilesPanel.spec.ts
git commit -m "web: pr changed-files panel"
```

---

## Task 8: Branches overview

**Files:** `web/src/views/BranchesView.vue`, `web/src/views/__tests__/BranchesView.spec.ts`; modify `web/src/router/index.ts`, `web/src/views/BuildsView.vue`.

- [ ] **Step 1: Create `web/src/views/BranchesView.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorMessage } from '@/plugins/http'
import { is401, loginUrl } from '@/lib/auth'
import { useBuildStore } from '@/stores/build'
import { formatPercent, coverageColor } from '@/lib/coverage'
import type { Build } from '@/types/build'

const route = useRoute()
const router = useRouter()
const store = useBuildStore()
const loading = ref(false)
const error = ref('')
const needLogin = ref(false)

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const repoRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}`)
const signInUrl = computed(() => loginUrl(route.fullPath))

// Latest (highest number) build per branch.
const branches = computed<Build[]>(() => {
  const latest = new Map<string, Build>()
  for (const b of store.list) {
    const cur = latest.get(b.branch)
    if (!cur || b.number > cur.number) latest.set(b.branch, b)
  }
  return Array.from(latest.values()).sort((a, b) => a.branch.localeCompare(b.branch))
})

async function load() {
  loading.value = true
  error.value = ''
  needLogin.value = false
  try {
    await store.fetchList(repoPath.value)
  } catch (e) {
    if (is401(e)) needLogin.value = true
    else error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h2 class="text-h5 mb-4">{{ namespace }}/{{ name }} · Branches</h2>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-card v-else-if="needLogin" class="pa-6 text-center">
      <p class="mb-4">Sign in to view this repository.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <v-table v-else hover>
      <thead><tr><th>Branch</th><th>Coverage</th><th>Build</th></tr></thead>
      <tbody>
        <tr v-for="b in branches" :key="b.branch" style="cursor: pointer" @click="router.push(`${repoRoute}/builds/${b.number}`)">
          <td>{{ b.branch }}</td>
          <td><v-chip :color="coverageColor(b.coverage)" size="small" label>{{ formatPercent(b.coverage) }}</v-chip></td>
          <td>#{{ b.number }}</td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>
```

- [ ] **Step 2: Add the route + a Branches link**

`web/src/router/index.ts`: add `import BranchesView from '@/views/BranchesView.vue'` and the route (next to the repo route):
```ts
    { path: '/report/:scm/:namespace/:name/branches', name: 'branches', component: BranchesView },
```
`web/src/views/BuildsView.vue`: in the header `div` (which already has the settings gear after `<v-spacer />`), add a Branches button before the gear:
```vue
      <v-btn variant="text" :to="`/report/${scm}/${namespace}/${name}/branches`">Branches</v-btn>
```

- [ ] **Step 3: Write `web/src/views/__tests__/BranchesView.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import BranchesView from '@/views/BranchesView.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))
vi.mock('axios', () => ({ default: { isAxiosError: (e: any) => !!e?.isAxiosError } }))

const vuetify = createVuetify({ components })
function makeRouter() {
  return createRouter({ history: createMemoryHistory(), routes: [{ path: '/report/:scm/:namespace/:name/branches', component: BranchesView }, { path: '/:p(.*)*', component: { template: '<div/>' } }] })
}

const mk = (number: number, branch: string, coverage: number) => ({
  number, branch, coverage, serviceName: '', serviceNumber: '', commit: 'c', pullRequest: 0, status: 'done',
  parallel: false, coverageChange: 0, baseBuildID: 0, baseBuildNumber: 0, commitMessage: '', authorName: '',
  authorEmail: '', createdAt: '2020-01-01T00:00:00Z', finishedAt: ''
})

describe('BranchesView', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('shows the latest build per branch', async () => {
    ;(http.get as any).mockResolvedValue({ data: [mk(3, 'main', 0.8), mk(1, 'main', 0.5), mk(2, 'dev', 0.6)] })
    const router = makeRouter()
    router.push('/report/github/o/r/branches')
    await router.isReady()
    const w = mount(BranchesView, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('main')
    expect(w.text()).toContain('dev')
    expect(w.text()).toContain('80.0%') // main's latest (#3), not #1's 50%
    expect(w.text()).not.toContain('50.0%')
  })
})
```

- [ ] **Step 4: Run + commit**

Run: `cd web && npx vitest run src/views/__tests__/BranchesView.spec.ts` → PASS.
```bash
cd .. && git add web/src/views/BranchesView.vue web/src/views/__tests__/BranchesView.spec.ts web/src/router/index.ts web/src/views/BuildsView.vue
git commit -m "web: branches overview"
```

---

## Task 9: Wire build detail (toggle + jobs + PR panel) and final verification

**Files:** `web/src/views/BuildDetailView.vue`.

- [ ] **Step 1: Update `BuildDetailView.vue`**

Imports (add): `import { ref } from 'vue'` (already imports `ref`); add component imports:
```ts
import FileCoverageTree from '@/components/FileCoverageTree.vue'
import JobsPanel from '@/components/JobsPanel.vue'
import ChangedFilesPanel from '@/components/ChangedFilesPanel.vue'
```
Add a view-mode ref in `<script setup>`:
```ts
const fileView = ref<'flat' | 'tree'>('flat')
```
Replace the success `<template v-else-if="store.current">` block with:
```vue
    <template v-else-if="store.current">
      <BuildSummary :build="store.current" />
      <JobsPanel v-if="store.current.jobs && store.current.jobs.length" :jobs="store.current.jobs" />
      <ChangedFilesPanel
        v-if="store.current.pullRequest > 0"
        :build="store.current"
        :repo-path="repoPath"
        :build-route="buildRoute"
        :login-redirect="$route.fullPath"
      />
      <div class="d-flex justify-end mb-2">
        <v-btn-toggle v-model="fileView" density="compact" mandatory>
          <v-btn value="flat" size="small">Flat</v-btn>
          <v-btn value="tree" size="small">Tree</v-btn>
        </v-btn-toggle>
      </div>
      <FileCoverageTable v-if="fileView === 'flat'" :build="store.current" :base="store.base" :build-route="buildRoute" />
      <FileCoverageTree v-else :build="store.current" :build-route="buildRoute" />
    </template>
```
(`repoPath` and `buildRoute` computeds already exist in this view. `loginUrl` is used inside `ChangedFilesPanel`, so it imports it itself — `ChangedFilesPanel` already imports `loginUrl`.)

- [ ] **Step 2: Build + extend the existing build-detail test (optional sanity)**

The existing `BuildDetailView.spec.ts` mounts a build with no jobs and `pullRequest: 0`, so `JobsPanel`/`ChangedFilesPanel` won't render and the test still passes. Run it to confirm no regression.

- [ ] **Step 3: Full verification**

```bash
cd web
npx vitest run
npm run lint
npm run build
cd ..
go build ./core/... ./models/... ./modules/... ./routers/...
go test ./routers/api/build/ ./modules/build/ -count=1
```
Expected: all tests pass (B1+B2+B3a+B3b); lint exit 0; vue-tsc + vite build succeed; backend builds + tests pass. `git status` shows no `web/dist`, `web/coverage`, `web/node_modules`, `web/web_gen.go`.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/BuildDetailView.vue
git commit -m "web: build detail tree toggle, jobs and pr panels"
```

---

## Self-Review notes (spec coverage)

- **B3b.1 job flags** → Task 1.
- **B3b.2 changed-files endpoint** → Task 2.
- **B3b.3 folder tree** → Tasks 4 (util) + 5 (component) + 9 (toggle).
- **B3b.4 branches overview** → Task 8.
- **B3b.5 jobs + flags panel** → Task 6 + 9 (wired).
- **B3b.6 PR changed-files panel** → Task 7 + 9 (wired).
- **B3b.7 data layer + testing** → Task 3 (types/store) + tests across tasks.
