# Homepage Stats, First-Login Sync & Account/Token Polish — Design

**Date:** 2026-06-15
**Branch:** `feat/home-account-polish` (off `shuenhoy/master`)

## Goal

Make the self-hosted instance friendlier right after login:
1. **First-login auto-sync** — a brand-new user's repos sync automatically instead of requiring a manual Sync click.
2. **Logged-in homepage** — replace the generic "Get started" hero with coverage stats and the top repositories for the signed-in user.
3. **Account page polish** — fix the misaligned cards on the account page.
4. **Repo token clarity** — the token field is intentionally blank (hidden secret); add a caption that says so.
5. **32-character repo tokens** — generate 32-char tokens for new repositories (currently 40).

These are independent, small changes bundled into one effort.

## Context

- **Sync today:** `Dashboard.vue` calls `repositoryStore.fetchList()` (GET `/api/v1/user/repos`, reads DB) on mount and exposes a manual **Sync** button → `synchronize()` (PATCH `/api/v1/user/repos` → `RepoService.Synchronize`, pulls from the SCM). A fresh user sees an empty list until they click Sync.
- **Home today:** `web/src/views/Home.vue` shows the same "Get started" → `/repos` hero to everyone. `useUserStore().isAuthenticated` (`current !== null`, populated by `fetch()` → GET `/api/v1/user`) tells us if someone is logged in.
- **Repo coverage:** `core.Repo` (and the JSON list) has no coverage. The badge resolves a repo's coverage via `buildStore.LatestOnBranch(repo.ID, repo.Branch, 0)` → `build.Coverage` (ratio 0..1) — see `routers/api/report/badge.go:29`. We reuse that.
- **User repos backend:** `routers/api/user/repo.go` — `HandleListRepo(userStore)` uses `userStore.ListRepositories(user) ([]*core.Repo, error)`. The authed user route group is in `routers/api/api.go` (`g.GET("/repos", checkLogin, …)`); `api.Router` already holds `UserStore` and `BuildStore`.
- **Token today:** `core.Repo.Token` is `json:"-"` (never serialized — security). It's revealed only via authed `GET /repos/.../token` (frontend `useRepoToken` composable; the SettingsView "Upload token" card has Show/Rotate buttons). `models.RepoStore.Create` sets `Token: util.GenerateToken()`. `modules/util/token.go` `GenerateToken()` makes 20 random bytes → 40 hex chars.
- **Account page:** `web/src/views/User.vue` lays the profile card and `AccountBindings` card in two `v-col cols=12 md=6` columns; the cards have different content heights and don't stretch, so they look ragged.
- Coverage color/format helpers exist in `web/src/lib/coverage.ts` (used by build views).

## Components

### 1. Backend — repo-stats endpoint

`routers/api/user/stats.go` (new) — `HandleRepoStats(userStore core.UserStore, buildStore core.BuildStore) gin.HandlerFunc`:

```go
type repoCoverage struct {
	SCM       core.SCMProvider `json:"scm"`
	NameSpace string           `json:"namespace"`
	Name      string           `json:"name"`
	ReportID  string           `json:"reportID"`
	Coverage  float64          `json:"coverage"` // ratio 0..1
}

type repoStats struct {
	RepoCount       int            `json:"repoCount"`
	ActivatedCount  int            `json:"activatedCount"`
	AverageCoverage float64        `json:"averageCoverage"` // ratio 0..1, over repos with a build
	TopRepos        []repoCoverage `json:"topRepos"`
}
```

Logic:
- `repos, err := userStore.ListRepositories(user)`; on error → `500` with an empty `repoStats{}`.
- `RepoCount = len(repos)`; `ActivatedCount = count(r.ReportID != "")`.
- For each repo, `build, err := buildStore.LatestOnBranch(r.ID, r.Branch, 0)`; if `err == nil && build != nil`, record a `repoCoverage{..., Coverage: build.Coverage}` and add to a running sum/count for the average.
- `AverageCoverage = sum / countWithBuilds` (0 when none).
- `TopRepos`: the recorded `repoCoverage` entries sorted by `Coverage` descending, first 5 (fewer if not enough). Stable: ties keep input order.
- `c.JSON(200, stats)`.

Wire in `routers/api/api.go` authed user group: `g.GET("/stats", checkLogin, user.HandleRepoStats(r.UserStore, r.BuildStore))`.

**Test** (`routers/api/user/stats_test.go`, gomock `MockUserStore`+`MockBuildStore`): repos with/without builds; assert `RepoCount`, `ActivatedCount`, `AverageCoverage` (only over repos with builds), and that `TopRepos` is sorted desc and capped at 5 and excludes repos with no build.

### 2. Backend — 32-character token

`modules/util/token.go`: change `make([]byte, 20)` → `make([]byte, 16)` so `hex.EncodeToString` yields **32** chars. Update the existing token test (assert `len == 32`). Only affects newly created repos and rotations; existing tokens are untouched.

### 3. Frontend — repository store additions

`web/src/stores/repository.ts`:
- `RepoStats` type (in `web/src/types/index.ts`): `{ repoCount: number; activatedCount: number; averageCoverage: number; topRepos: TopRepo[] }`, `TopRepo { scm: SCM; namespace: string; name: string; reportID: string; coverage: number }`.
- `stats = ref<RepoStats | null>(null)`; `async fetchStats()` → GET `/api/v1/user/stats` → `stats.value = data`.
- `autoSynced = ref(false)`; `async ensureSynced()`:
  ```ts
  if (autoSynced.value) return
  autoSynced.value = true
  await fetchList()
  if (list.value.length === 0) {
    await synchronize()
    await fetchList()
  }
  ```
  Guarded so it runs at most once per session (avoids re-syncing a user who genuinely has zero repos).

### 4. Frontend — homepage

`web/src/views/Home.vue`:
- Use `useUserStore()` + `useRepositoryStore()`. On mount: if authenticated, `await ensureSynced()` then `await fetchStats()`.
- **Guest** (`!isAuthenticated`): the existing hero (title, tagline, "Get started" → `/repos`). Unchanged.
- **Authenticated:**
  - A row of three summary tiles (Vuetify cards): **Repositories** = `repoCount`, **Activated** = `activatedCount`, **Avg coverage** = `formatPercent(averageCoverage)`.
  - A **Top repositories** card: list the up-to-5 `topRepos`, each a clickable row showing `namespace/name` + a coverage chip (`coverageColor`/`formatPercent` from `lib/coverage`), linking to `/report/<scm>/<namespace>/<name>/builds`.
  - Empty state when `topRepos` is empty: a short "No coverage yet — activate a repository" with a button to `/repos`.
  - A busy indicator while `ensureSynced()`/`fetchStats()` run.

### 5. Frontend — Dashboard auto-sync

`web/src/views/Dashboard.vue`: in `load()`, call `await store.ensureSynced()` (which performs the empty-list auto-sync) instead of a bare `fetchList()`. The manual **Sync** button stays. The existing busy spinner covers the auto-sync.

### 6. Frontend — account page alignment

`web/src/views/User.vue`: make the two columns equal height — `<v-row justify="center" align="stretch">` and add `h-100` to the profile `v-card`. In `web/src/components/AccountBindings.vue`, add `h-100` to its root `v-card` so both cards fill the row height and line up.

### 7. Frontend — token card caption

`web/src/views/SettingsView.vue` "Upload token" card: add a one-line caption under the title, e.g. *"Hidden for security — click to reveal."* No logic change; clarifies that a blank field is expected.

## Data flow

1. User logs in → app bootstraps `user.fetch()`.
2. Lands on Home (or Dashboard) → `ensureSynced()` runs once: if the DB has no repos for the user, it syncs from the SCM, then reloads the list.
3. Home (authenticated) → `fetchStats()` → renders tiles + top repos from `/api/v1/user/stats`.
4. New repo activation → server generates a 32-char upload token (hidden; revealed on demand).

## Error handling

- `fetchStats()` / `ensureSynced()` failures: surfaced via the page's existing snackbar/error path; the homepage still renders (tiles show zeros / guest-style fallback). A sync failure does not blank the page.
- Stats endpoint: store error → 500 with empty stats; repos with no build are simply excluded from average/top (not errors).
- `LatestOnBranch` returning not-found for a repo is expected (no builds yet) and skipped.

## Testing

- **Backend:** `stats_test.go` (counts, average over repos-with-builds, top-5 desc sort, exclude no-build repos); `token` test updated to assert length 32.
- **Frontend:**
  - `Home.spec.ts`: authenticated → renders tiles + top repos from a mocked stats response; guest → renders "Get started".
  - Repository store test: `ensureSynced()` syncs when the list is empty and does **not** sync when non-empty, and runs only once (second call is a no-op).
  - Existing `RepoSetting` fixtures unaffected (no new setting fields).
- Full suite + lint + type-check build green.

## Out of scope

- Coverage on the main Dashboard repo list (stats stay on the homepage).
- Per-repo coverage history/sparklines on the homepage.
- Changing the token reveal/rotate mechanism (only a clarifying caption + length).
- Re-issuing tokens for existing repos (only new/rotated tokens become 32-char).
