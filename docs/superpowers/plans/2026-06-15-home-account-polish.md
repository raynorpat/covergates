# Homepage Stats, First-Login Sync & Account/Token Polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auto-sync a new user's repos on first login, show coverage stats + top repos on the logged-in homepage, fix the account page card alignment, clarify the (intentionally hidden) repo token, and make new repo tokens 32 chars.

**Architecture:** A new authed `GET /api/v1/user/stats` endpoint aggregates per-repo latest coverage (via the badge's `LatestOnBranch` lookup). The repository store gains `fetchStats()` and a once-per-session `ensureSynced()`; `Home.vue` renders stats for authenticated users and the Dashboard/Home call `ensureSynced()`. Plus small Go (token length) and Vue (card alignment, token caption) tweaks.

**Tech Stack:** Go 1.14, Gin, GORM, gomock; Vue 3 + Vuetify 3 + Pinia + Vitest. No new dependencies.

---

## Environment notes (read first)

- Build/test Go with `CGO_ENABLED=0` from the **repo root** `D:\raynorpat\covergates`; cwd persists across Bash calls — never `cd` into a subdir for go commands. Frontend commands run from `web/`.
- `core.Build.Coverage` is a ratio in `[0,1]`. `formatPercent`/`coverageColor` (in `web/src/lib/coverage.ts`) take a ratio.
- Go handler tests use the external `package user_test`, set the user via `request.WithUser(c, u)` in a gin middleware, and assert by parsing the JSON response body (see `routers/api/user/repo_test.go`). `mock.NewMockUserStore` and `mock.NewMockBuildStore` exist.
- Frontend tests mock http via `vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn(), post: vi.fn() }, errorMessage: (e) => String(e) }))` and mount with `createVuetify({components,directives})` + a memory router + a fresh pinia (see `web/src/views/__tests__/Dashboard.spec.ts`).

## File Structure

**Create:**
- `routers/api/user/stats.go` — `HandleRepoStats` + the `repoStats`/`repoCoverage` response structs.
- `routers/api/user/stats_test.go` — handler test.
- `web/src/views/__tests__/Home.spec.ts` — homepage test.

**Modify:**
- `modules/util/token.go` — 20→16 bytes; `modules/util/token_test.go` — assert len 32.
- `routers/api/api.go` — register the `/stats` route.
- `web/src/types/index.ts` — `RepoStats`/`TopRepo` types.
- `web/src/stores/repository.ts` — `stats`, `fetchStats()`, `autoSynced`, `ensureSynced()`.
- `web/src/stores/__tests__/repository.spec.ts` — `ensureSynced` tests.
- `web/src/views/Home.vue` — authed homepage.
- `web/src/views/Dashboard.vue` — call `ensureSynced()`.
- `web/src/views/User.vue` + `web/src/components/AccountBindings.vue` — equal-height cards.
- `web/src/views/SettingsView.vue` — token caption.

---

## Task 1: 32-character repo token

**Files:**
- Modify: `modules/util/token.go`
- Test: `modules/util/token_test.go`

- [ ] **Step 1: Update the test to expect 32 chars**

In `modules/util/token_test.go`, change the length assertion from 40 to 32:

```go
	if len(a) != 32 {
		t.Fatalf("token length = %d, want 32", len(a))
	}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/util/ -run TestGenerateToken`
Expected: FAIL — `token length = 40, want 32`.

- [ ] **Step 3: Shorten the token to 16 bytes**

In `modules/util/token.go`, change `make([]byte, 20)` to `make([]byte, 16)` (16 bytes → 32 hex chars):

```go
func GenerateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/util/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/util/token.go modules/util/token_test.go
git commit -m "repo: generate 32-character upload tokens"
```

---

## Task 2: Repo-stats endpoint

**Files:**
- Create: `routers/api/user/stats.go`
- Create: `routers/api/user/stats_test.go`
- Modify: `routers/api/api.go`

- [ ] **Step 1: Write the failing handler test**

Create `routers/api/user/stats_test.go`:

```go
package user_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/covergates/covergates/routers/api/user"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"gorm.io/gorm"
)

func TestHandleRepoStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u := &core.User{Login: "test"}
	repos := []*core.Repo{
		{ID: 1, NameSpace: "o", Name: "a", SCM: core.Github, ReportID: "r1", Branch: "main"}, // cov 0.90
		{ID: 2, NameSpace: "o", Name: "b", SCM: core.Github, ReportID: "r2", Branch: "main"}, // cov 0.50
		{ID: 3, NameSpace: "o", Name: "c", SCM: core.Github, ReportID: "", Branch: "main"},   // not activated, no build
	}
	userStore := mock.NewMockUserStore(ctrl)
	buildStore := mock.NewMockBuildStore(ctrl)
	userStore.EXPECT().ListRepositories(u).Return(repos, nil)
	buildStore.EXPECT().LatestOnBranch(uint(1), "main", uint(0)).Return(&core.Build{Coverage: 0.90}, nil)
	buildStore.EXPECT().LatestOnBranch(uint(2), "main", uint(0)).Return(&core.Build{Coverage: 0.50}, nil)
	buildStore.EXPECT().LatestOnBranch(uint(3), "main", uint(0)).Return(nil, gorm.ErrRecordNotFound)

	r := gin.New()
	r.Use(func(c *gin.Context) { request.WithUser(c, u) })
	r.GET("/user/stats", user.HandleRepoStats(userStore, buildStore))

	req, _ := http.NewRequest("GET", "/user/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got struct {
		RepoCount       int     `json:"repoCount"`
		ActivatedCount  int     `json:"activatedCount"`
		AverageCoverage float64 `json:"averageCoverage"`
		TopRepos        []struct {
			Name     string  `json:"name"`
			Coverage float64 `json:"coverage"`
		} `json:"topRepos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got.RepoCount != 3 || got.ActivatedCount != 2 {
		t.Fatalf("counts: repo=%d activated=%d", got.RepoCount, got.ActivatedCount)
	}
	if got.AverageCoverage < 0.69 || got.AverageCoverage > 0.71 { // (0.9+0.5)/2 over repos WITH a build
		t.Fatalf("average = %v, want ~0.70", got.AverageCoverage)
	}
	if len(got.TopRepos) != 2 || got.TopRepos[0].Name != "a" || got.TopRepos[1].Name != "b" {
		t.Fatalf("topRepos wrong: %+v", got.TopRepos) // repo c (no build) excluded; sorted desc by coverage
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./routers/api/user/ -run TestHandleRepoStats`
Expected: FAIL — `undefined: user.HandleRepoStats`.

- [ ] **Step 3: Implement the handler**

Create `routers/api/user/stats.go`:

```go
package user

import (
	"sort"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
)

type repoCoverage struct {
	SCM       core.SCMProvider `json:"scm"`
	NameSpace string           `json:"namespace"`
	Name      string           `json:"name"`
	ReportID  string           `json:"reportID"`
	Coverage  float64          `json:"coverage"`
}

type repoStats struct {
	RepoCount       int            `json:"repoCount"`
	ActivatedCount  int            `json:"activatedCount"`
	AverageCoverage float64        `json:"averageCoverage"`
	TopRepos        []repoCoverage `json:"topRepos"`
}

// HandleRepoStats summarizes the user's repositories and their coverage.
// @Summary Repository coverage stats for the user
// @Tags User
// @Success 200 {object} repoStats
// @Router /user/stats [get]
func HandleRepoStats(userStore core.UserStore, buildStore core.BuildStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := request.MustGetUserFrom(c)
		repos, err := userStore.ListRepositories(user)
		if err != nil {
			c.JSON(500, &repoStats{})
			return
		}
		stats := &repoStats{RepoCount: len(repos)}
		withCoverage := make([]repoCoverage, 0, len(repos))
		var sum float64
		for _, repo := range repos {
			if repo.ReportID != "" {
				stats.ActivatedCount++
			}
			build, err := buildStore.LatestOnBranch(repo.ID, repo.Branch, 0)
			if err != nil || build == nil {
				continue
			}
			withCoverage = append(withCoverage, repoCoverage{
				SCM:       repo.SCM,
				NameSpace: repo.NameSpace,
				Name:      repo.Name,
				ReportID:  repo.ReportID,
				Coverage:  build.Coverage,
			})
			sum += build.Coverage
		}
		if len(withCoverage) > 0 {
			stats.AverageCoverage = sum / float64(len(withCoverage))
		}
		sort.SliceStable(withCoverage, func(i, j int) bool {
			return withCoverage[i].Coverage > withCoverage[j].Coverage
		})
		if len(withCoverage) > 5 {
			withCoverage = withCoverage[:5]
		}
		stats.TopRepos = withCoverage
		c.JSON(200, stats)
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./routers/api/user/`
Expected: PASS.

- [ ] **Step 5: Register the route**

In `routers/api/api.go`, in the authed user group (right after the `g.GET("/repos", ...)` line), add:

```go
		g.GET("/repos", checkLogin, user.HandleListRepo(r.UserStore))
		g.GET("/stats", checkLogin, user.HandleRepoStats(r.UserStore, r.BuildStore))
```

- [ ] **Step 6: Build to verify wiring**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./routers/...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
cd /d/raynorpat/covergates
git add routers/api/user/stats.go routers/api/user/stats_test.go routers/api/api.go
git commit -m "user: repository coverage stats endpoint"
```

---

## Task 3: Frontend types + store (fetchStats, ensureSynced)

**Files:**
- Modify: `web/src/types/index.ts`
- Modify: `web/src/stores/repository.ts`
- Test: `web/src/stores/__tests__/repository.spec.ts`

- [ ] **Step 1: Add the stats types**

In `web/src/types/index.ts`, append:

```ts
export interface TopRepo {
  scm: SCM
  namespace: string
  name: string
  reportID: string
  coverage: number
}

export interface RepoStats {
  repoCount: number
  activatedCount: number
  averageCoverage: number
  topRepos: TopRepo[]
}
```

- [ ] **Step 2: Write the failing store test**

Add to `web/src/stores/__tests__/repository.spec.ts` (read the file first to match its imports/mock setup; it mocks `@/plugins/http`). Add these tests inside the existing `describe`:

```ts
  it('ensureSynced syncs when the list is empty, once per session', async () => {
    ;(http.get as any).mockResolvedValue({ data: [] }) // empty list
    ;(http.patch as any).mockResolvedValue({ data: 'ok' })
    const store = useRepositoryStore()
    await store.ensureSynced()
    expect(http.patch).toHaveBeenCalledWith('/api/v1/user/repos') // synced because empty
    ;(http.patch as any).mockClear()
    await store.ensureSynced() // second call is a no-op (once guard)
    expect(http.patch).not.toHaveBeenCalled()
  })

  it('ensureSynced does not sync when repos already exist', async () => {
    ;(http.get as any).mockResolvedValue({ data: [
      { ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'a', Branch: 'm', Private: false, SCM: 'github' }
    ] })
    ;(http.patch as any).mockResolvedValue({ data: 'ok' })
    const store = useRepositoryStore()
    await store.ensureSynced()
    expect(http.patch).not.toHaveBeenCalled()
  })

  it('fetchStats loads the stats endpoint', async () => {
    const stats = { repoCount: 2, activatedCount: 1, averageCoverage: 0.8, topRepos: [] }
    ;(http.get as any).mockResolvedValue({ data: stats })
    const store = useRepositoryStore()
    await store.fetchStats()
    expect(http.get).toHaveBeenCalledWith('/api/v1/user/stats')
    expect(store.stats).toEqual(stats)
  })
```

Ensure the file's `vi.mock('@/plugins/http', ...)` includes `patch: vi.fn()` (the existing repository.spec already mocks `post`; add `patch` and `get` if missing). Add `beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })` if not present so the `autoSynced` guard resets per test (a fresh pinia gives a fresh store).

- [ ] **Step 3: Run to verify it fails**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/stores/__tests__/repository.spec.ts`
Expected: FAIL — `store.ensureSynced is not a function` / `store.stats` undefined.

- [ ] **Step 4: Implement the store additions**

Edit `web/src/stores/repository.ts`:

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Repository, RepoStats } from '@/types'
import type { RepoSetting } from '@/types/setting'

export const useRepositoryStore = defineStore('repository', () => {
  const list = ref<Repository[]>([])
  const setting = ref<RepoSetting | null>(null)
  const stats = ref<RepoStats | null>(null)
  const autoSynced = ref(false)

  async function fetchList() {
    const { data } = await http.get<Repository[]>('/api/v1/user/repos')
    list.value = data ?? []
  }

  async function synchronize() {
    await http.patch('/api/v1/user/repos')
    await fetchList()
  }

  // ensureSynced syncs the user's repos from the SCM the first time they have
  // none, once per session. Returning users with repos are left untouched.
  async function ensureSynced() {
    if (autoSynced.value) return
    autoSynced.value = true
    await fetchList()
    if (list.value.length === 0) {
      await synchronize()
    }
  }

  async function fetchStats() {
    const { data } = await http.get<RepoStats>('/api/v1/user/stats')
    stats.value = data
  }

  async function fetchSetting(repoPath: string) {
    const { data } = await http.get<RepoSetting>(`${repoPath}/setting`)
    setting.value = data
  }

  async function updateSetting(repoPath: string, value: RepoSetting) {
    const { data } = await http.post<RepoSetting>(`${repoPath}/setting`, value)
    setting.value = data
  }

  return { list, setting, stats, fetchList, synchronize, ensureSynced, fetchStats, fetchSetting, updateSetting }
})
```

(Note: `synchronize()` already calls `fetchList()`, so `ensureSynced` doesn't need a second `fetchList` after sync.)

- [ ] **Step 5: Run to verify it passes**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/stores/__tests__/repository.spec.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/types/index.ts web/src/stores/repository.ts web/src/stores/__tests__/repository.spec.ts
git commit -m "web: repository store stats and first-login auto-sync"
```

---

## Task 4: Logged-in homepage

**Files:**
- Modify: `web/src/views/Home.vue`
- Test: `web/src/views/__tests__/Home.spec.ts`

- [ ] **Step 1: Write the failing homepage test**

Create `web/src/views/__tests__/Home.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import Home from '@/views/Home.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [
  { path: '/', component: { template: '<div/>' } },
  { path: '/repos', component: { template: '<div/>' } },
  { path: '/report/:scm/:namespace/:name/builds', component: { template: '<div/>' } }
] })

function routeGet(map: Record<string, unknown>) {
  ;(http.get as any).mockImplementation((url: string) => {
    for (const key of Object.keys(map)) {
      if (url.endsWith(key)) return Promise.resolve({ data: map[key] })
    }
    return Promise.resolve({ data: null })
  })
}

describe('Home', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('shows stats and top repos when authenticated', async () => {
    routeGet({
      '/api/v1/user': { login: 'me', email: 'me@x.com' },
      '/api/v1/user/repos': [{ ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'a', Branch: 'm', Private: false, SCM: 'github' }],
      '/api/v1/user/stats': { repoCount: 3, activatedCount: 2, averageCoverage: 0.842, topRepos: [
        { scm: 'github', namespace: 'o', name: 'a', reportID: 'x', coverage: 0.9 }
      ] }
    })
    const w = mount(Home, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('o/a')      // top repo
    expect(w.text()).toContain('84.2%')    // average coverage tile
    expect(w.text()).not.toContain('Get started')
  })

  it('shows the get-started hero when not authenticated', async () => {
    routeGet({ '/api/v1/user': null })
    const w = mount(Home, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('Get started')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/views/__tests__/Home.spec.ts`
Expected: FAIL — the authenticated branch / stats UI does not exist yet.

- [ ] **Step 3: Implement Home.vue**

Replace `web/src/views/Home.vue` with:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useRepositoryStore } from '@/stores/repository'
import { formatPercent, coverageColor } from '@/lib/coverage'

const router = useRouter()
const user = useUserStore()
const repos = useRepositoryStore()
const busy = ref(false)

onMounted(async () => {
  if (!user.current) await user.fetch()
  if (!user.isAuthenticated) return
  busy.value = true
  try {
    await repos.ensureSynced()
    await repos.fetchStats()
  } finally {
    busy.value = false
  }
})

function openRepo(r: { scm: string; namespace: string; name: string }) {
  router.push(`/report/${r.scm}/${r.namespace}/${r.name}/builds`)
}
</script>

<template>
  <v-container class="py-10">
    <!-- Guest hero -->
    <v-row v-if="!user.isAuthenticated" justify="center">
      <v-col cols="12" md="8" class="text-center">
        <h1 class="text-h3 mb-4">Covergates</h1>
        <p class="text-body-1 mb-8">Self-hosted coverage reports, coveralls-compatible.</p>
        <v-btn color="primary" size="large" @click="router.push('/repos')">Get started</v-btn>
      </v-col>
    </v-row>

    <!-- Authenticated dashboard -->
    <template v-else>
      <h1 class="text-h4 mb-6">Welcome, {{ user.current?.login }}</h1>
      <v-progress-linear v-if="busy" indeterminate color="primary" class="mb-4" />
      <v-row v-if="repos.stats" class="mb-2">
        <v-col cols="12" sm="4">
          <v-card class="pa-4 h-100">
            <div class="text-overline">Repositories</div>
            <div class="text-h4">{{ repos.stats.repoCount }}</div>
          </v-card>
        </v-col>
        <v-col cols="12" sm="4">
          <v-card class="pa-4 h-100">
            <div class="text-overline">Activated</div>
            <div class="text-h4">{{ repos.stats.activatedCount }}</div>
          </v-card>
        </v-col>
        <v-col cols="12" sm="4">
          <v-card class="pa-4 h-100">
            <div class="text-overline">Avg coverage</div>
            <div class="text-h4">{{ formatPercent(repos.stats.averageCoverage) }}</div>
          </v-card>
        </v-col>
      </v-row>

      <v-card v-if="repos.stats" class="pa-4">
        <div class="text-h6 mb-2">Top repositories</div>
        <v-table v-if="repos.stats.topRepos.length" density="compact">
          <tbody>
            <tr v-for="r in repos.stats.topRepos" :key="`${r.namespace}/${r.name}`" style="cursor: pointer" @click="openRepo(r)">
              <td><code>{{ r.namespace }}/{{ r.name }}</code></td>
              <td class="text-right">
                <v-chip :color="coverageColor(r.coverage)" size="small" label>{{ formatPercent(r.coverage) }}</v-chip>
              </td>
            </tr>
          </tbody>
        </v-table>
        <p v-else class="text-body-2 text-medium-emphasis mb-0">
          No coverage yet — <a href="#" @click.prevent="router.push('/repos')">activate a repository</a>.
        </p>
      </v-card>
    </template>
  </v-container>
</template>
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/views/__tests__/Home.spec.ts`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/views/Home.vue web/src/views/__tests__/Home.spec.ts
git commit -m "web: logged-in homepage with coverage stats and top repos"
```

---

## Task 5: Dashboard first-login auto-sync

**Files:**
- Modify: `web/src/views/Dashboard.vue`

- [ ] **Step 1: Auto-sync on mount; keep plain refetch for activation**

`load()` is also the `@activated` reload handler (RepoList emits `activated` to refresh after a repo is activated). It must keep doing a plain `fetchList()` every time, or the once-guarded `ensureSynced()` would make post-activation refreshes no-ops. So add a separate mount-time `init()` that auto-syncs, and leave `load()` as the refetch.

In `web/src/views/Dashboard.vue`, read the current `load()`/`onMounted(load)`. Keep `load()` as a `fetchList()` refresh and change the mount call:

```ts
async function load() {
  busy.value = true
  try { await store.fetchList() } catch (e) { snackbar.value = errorMessage(e) } finally { busy.value = false }
}

async function init() {
  busy.value = true
  try { await store.ensureSynced() } catch (e) { snackbar.value = errorMessage(e) } finally { busy.value = false }
}

onMounted(init)
```

Leave `@activated="load"` on `<RepoList>` and the manual **Sync** button / `sync()` handler unchanged. (`onMounted(load)` becomes `onMounted(init)`; `load` stays for the `@activated` handler.)

- [ ] **Step 2: Run the dashboard test to confirm no regression**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/views/__tests__/Dashboard.spec.ts`
Expected: PASS. (The existing test mocks a non-empty list, so `init()` → `ensureSynced` → `fetchList` renders the list and does not sync; the search filter still works.)

- [ ] **Step 3: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/views/Dashboard.vue
git commit -m "web: auto-sync repositories on first dashboard load"
```

---

## Task 6: Account card alignment + token caption

**Files:**
- Modify: `web/src/views/User.vue`
- Modify: `web/src/components/AccountBindings.vue`
- Modify: `web/src/views/SettingsView.vue`

- [ ] **Step 1: Make the account cards equal height**

In `web/src/views/User.vue`, add `align="stretch"` to the row and `h-100` to the profile card:

```html
    <v-row justify="center" align="stretch">
      <v-col cols="12" md="6">
        <v-card class="pa-6 h-100">
```

In `web/src/components/AccountBindings.vue`, add `h-100` to its root `v-card`'s class (read the file first; append `h-100` to the existing class list on the outermost `<v-card>`).

- [ ] **Step 2: Add the token caption**

In `web/src/views/SettingsView.vue`, in the "Upload token" `v-card`, add a caption line right after the `<div class="text-h6 mb-2">Upload token</div>` title:

```html
        <div class="text-h6 mb-2">Upload token</div>
        <p class="text-body-2 text-medium-emphasis mb-2">Hidden for security — click to reveal.</p>
```

- [ ] **Step 3: Verify the frontend still builds + tests pass**

Run: `cd /d/raynorpat/covergates/web && npx vitest run && npm run lint`
Expected: all suites pass; lint clean.

- [ ] **Step 4: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/views/User.vue web/src/components/AccountBindings.vue web/src/views/SettingsView.vue
git commit -m "web: equal-height account cards and upload-token caption"
```

---

## Task 7: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Backend vet + build + targeted tests**

Run:
```
cd /d/raynorpat/covergates && CGO_ENABLED=0 go vet ./modules/util/... ./routers/api/user/... 2>&1 | tail -20
CGO_ENABLED=0 go build ./core/... ./modules/... ./routers/... ./cmd/... ./mock/...
CGO_ENABLED=0 go test ./modules/util/ ./routers/api/user/ ./routers/api/build/ ./modules/notify/
```
Expected: vet clean; build clean; all four packages `ok`. (`models` is excluded — needs CGO/sqlite.)

- [ ] **Step 2: Frontend lint, test, build**

Run:
```
cd /d/raynorpat/covergates/web && npm run lint && npx vitest run && npm run build
```
Expected: lint clean; all Vitest suites pass; production build succeeds (only the pre-existing chunk-size warning).

- [ ] **Step 3: Final commit (only if a verification fix was needed)**

```bash
cd /d/raynorpat/covergates
git add -A
git commit -m "polish: verification fixes"
```
(Skip if nothing changed.)

---

## Done criteria

- A brand-new user's repos sync automatically on first reaching Home or the Dashboard; returning users with repos are not auto-synced; the manual Sync button still works.
- The logged-in homepage shows Repositories / Activated / Avg coverage tiles and the top 5 repos by coverage (linking to their builds); guests still see the "Get started" hero.
- The account page's two cards are equal height/aligned.
- The Upload-token card shows a "Hidden for security — click to reveal" caption; newly created/rotated repo tokens are 32 chars.
- All targeted Go packages and the full frontend suite pass; `go vet` clean; the frontend type-check build succeeds.
