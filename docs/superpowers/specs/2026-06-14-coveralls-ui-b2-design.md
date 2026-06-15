# Sub-project B2 — Coverage Browsing UI (builds list, build detail, source view)

**Date:** 2026-06-14
**Status:** Approved design, ready for implementation planning
**Part of:** "Make covergates work like coveralls.io" — Sub-project B (UI), chunk **B2 of B1/B2/B3**

## Background

Sub-project A built the coveralls-compatible Build/Job model + read endpoints. B1 rebuilt
the frontend on Vue 3 / Vuetify 3 / Vite / Pinia / `<script setup>` and shipped the app
shell, auth, and the repo dashboard (with a placeholder repo page). B2 fills in the core
coverage-browsing experience on top of the Build API:

| Screen | Route | Source |
|---|---|---|
| Builds list (repo page) | `/report/:scm/:namespace/:name` | `GET /api/v1/repos/:scm/:namespace/:name/builds` |
| Build detail | `/report/:scm/:namespace/:name/builds/:number` | `GET .../builds/:number` |
| Per-line source | `/report/:scm/:namespace/:name/builds/:number/source/:path+` | build's `sourceFiles` + `GET .../content/*path?gitref=` |

### Relevant existing API (from A / B1)

- `GET /api/v1/repos/:scm/:namespace/:name/builds` → `Build[]` (no `sourceFiles`), public for
  public repos; 401 for private without access (`permittedRepo`).
- `GET .../builds/:number` → `Build` with merged `sourceFiles` + `jobs` (jobs without files).
- `GET .../content/*path?gitref=<commit>` → raw file bytes. **Requires login** (401 otherwise).
- `Build` JSON fields: `number, serviceName, serviceNumber, commit, branch, pullRequest,
  status, parallel, coverage, coverageChange, baseBuildID, commitMessage, authorName,
  authorEmail, sourceFiles[], jobs[], createdAt, finishedAt`.
- `SourceFile` JSON: `{ name, source_digest, coverage: (int|null)[] }` — index `i` = line
  `i+1`; `null` = non-executable; `0` = missed; `>0` = hit count.

## Decisions (from brainstorming, visual companion used)

1. **Structure:** repo page = builds list → build detail (summary + file list) → per-line source.
2. **File list:** flat **sortable, searchable table** (folder tree deferred to B3).
3. **Per-file delta:** **included** in B2 (shown in the build-detail file table).
4. **Source gutter:** show **hit count + covered/missed coloring**.
5. Visual style: Vuetify (Material), coveralls-like layout (not pixel-match).
6. Public repos: builds list + detail viewable without login; source view requires login
   (content endpoint is auth-gated) → show a sign-in prompt.

## Architecture

All new frontend code under `web/src`. One small backend change (expose the base build's
number) enables per-file deltas. No other backend changes.

### File structure

Frontend (create):
- `web/src/types/build.ts` — `Build`, `Job`, `SourceFile` TS types.
- `web/src/lib/coverage.ts` — pure helpers: `fileCoverage`, `lineState`, `formatPercent`,
  `coverageColor`.
- `web/src/lib/highlight.ts` — highlight.js wrapper (Vue 3, replaces the old Vue 2 plugin).
- `web/src/stores/build.ts` — Pinia store: list + current build + base build + source text.
- `web/src/views/BuildsView.vue` — builds list (repo page).
- `web/src/views/BuildDetailView.vue` — summary + file table.
- `web/src/views/SourceView.vue` — per-line source.
- `web/src/components/BuildList.vue` — builds table.
- `web/src/components/BuildSummary.vue` — coverage gauge + commit/job summary.
- `web/src/components/FileCoverageTable.vue` — per-file table (coverage % + Δ + lines).
- `web/src/components/SourceLines.vue` — per-line rendering (gutter + hit count + colour).
- tests under `web/src/**/__tests__/`.

Frontend (modify):
- `web/src/router/index.ts` — replace the `repo` placeholder route with the three B2 routes
  (`BuildsView`, `BuildDetailView`, `SourceView`); none `requiresAuth`.
- delete `web/src/views/RepoPlaceholder.vue` (replaced by `BuildsView`).
- `web/src/assets/styles/variables.scss` — reuse/define `statement-hit` / `statement-miss`
  coverage colors for `SourceLines`.

Backend (modify, small):
- `core/build.go` — add `BaseBuildNumber int` to `Build`.
- `models/build.go` — add `BaseBuildNumber` column; copy in `copyBuildToModel` + `ToCoreBuild`.
- `modules/build/service.go` — in `finalize`, when a base build is found set
  `build.BaseBuildNumber = base.Number` (alongside `BaseBuildID`).

### B2.2 Build store & utilities

`web/src/types/build.ts`:
```ts
export interface SourceFile { name: string; source_digest: string; coverage: (number | null)[] }
export interface Job { id: number; serviceJobID: string; coverage: number }
export interface Build {
  number: number; serviceName: string; serviceNumber: string; commit: string; branch: string
  pullRequest: number; status: 'processing' | 'done' | 'errored'; parallel: boolean
  coverage: number; coverageChange: number; baseBuildID: number; baseBuildNumber: number
  commitMessage: string; authorName: string; authorEmail: string
  sourceFiles?: SourceFile[]; jobs?: Job[]; createdAt: string; finishedAt: string
}
```

`web/src/lib/coverage.ts` (pure, unit-tested):
- `fileCoverage(f: SourceFile) → { covered: number; relevant: number; ratio: number }` —
  relevant = non-null entries; covered = entries `> 0`; ratio = covered/relevant (0 if none).
- `lineState(hits: number | null) → 'hit' | 'miss' | 'none'`.
- `formatPercent(ratio: number) → string` (e.g. `82.4%`).
- `coverageColor(ratio: number) → string` (green ≥ 0.8, amber ≥ 0.5, red below — a simple
  threshold used for chips/bars).

`web/src/stores/build.ts` (Pinia):
- state: `list: Build[]`, `current: Build | null`, `base: Build | null`, `source: string`.
- `fetchList(repoPath: string)` → `GET {repoPath}/builds`.
- `fetchBuild(repoPath, number)` → `GET {repoPath}/builds/:number`; then if
  `current.baseBuildNumber > 0`, `fetchBuild` the base into `base` (for per-file delta),
  else `base = null`.
- `fetchSource(repoPath, path, commit)` → `GET {repoPath}/content/{path}?gitref={commit}`
  (raw text). Surfaces 401 distinctly so the view shows a sign-in prompt.
- `repoPath` = `/api/v1/repos/${scm}/${namespace}/${name}` built by the caller.

### B2.3 Builds list (`BuildsView` + `BuildList`)

On mount, resolve the repo (reuse `repository` store's `Find` or a direct
`GET /api/v1/repos/:scm/:ns/:name`) for the header, and `build.fetchList`. Render a header
(repo name + link to `repo.URL`) and `BuildList`: a Vuetify `v-data-table` with columns
build #, branch/PR (show `PR #n → branch` when `pullRequest > 0` else `branch`), commit
(short SHA + first line of `commitMessage`), coverage % (colored chip), Δ (`coverageChange`
with up/down arrow), status, relative time (`createdAt`). A branch `v-select` filters
client-side over distinct branches in the loaded list. Row click → build detail. Empty
state when no builds.

### B2.4 Build detail (`BuildDetailView` + `BuildSummary` + `FileCoverageTable`)

`build.fetchBuild(repoPath, number)` (loads `current` + `base`). `BuildSummary`: a
`v-progress-circular` coverage gauge (or large %), Δ vs base (`coverageChange` + base build
link when `baseBuildNumber>0`), commit message/author/relative time, status, job count.
`FileCoverageTable`: for each `current.sourceFiles` entry compute `fileCoverage`; if `base`
present, look up the same file by name in `base.sourceFiles`, compute its ratio, and show
`Δ = current.ratio − base.ratio` (new file → Δ = current.ratio; file absent in current →
not shown). Columns: file path (link to source), coverage bar + %, covered/relevant lines,
Δ. Searchable + sortable. Handle private-repo 401 with an inline "sign in" message.

### B2.5 Source view (`SourceView` + `SourceLines`)

Needs both the file's coverage array and the source text. Resolve the build
(`build.current`, fetching if navigated to directly) to get `commit` + the `SourceFile`
matching `:path`. `build.fetchSource(repoPath, path, build.commit)`:
- On 401 → render a "Sign in to view source" card with a login link
  (`/login?redirect=<currentPath>`). (Coverage list/summary remain viewable; only source
  text is gated.)
- On success → `SourceLines` renders one row per line: line number, hit-count gutter
  (`{hits}×` when the line is executable, blank otherwise), and the code rendered with
  `highlight.js` (auto-detect or by file extension). Each row gets `statement-hit` /
  `statement-miss` / no class from `lineState(coverage[i])`. Coverage array index `i` maps
  to line `i+1`.
- If the file has no coverage entry in the build (e.g. not in report) → render source
  plainly with a notice.

`web/src/lib/highlight.ts`: wraps `highlight.js` (`hljs.highlightAuto` or
`hljs.highlight` with a language guessed from the extension), returns HTML; the GitHub
stylesheet is imported once. Splitting highlighted HTML into per-line strings follows the
B1-retained approach.

### B2.7 Backend: expose base build number

So the frontend can fetch the base build for per-file deltas:
- `core.Build` gains `BaseBuildNumber int \`json:"baseBuildNumber"\``.
- `models.Build` gains a `BaseBuildNumber int` column; `copyBuildToModel` and `ToCoreBuild`
  carry it.
- `modules/build/service.go` `finalize`: when `base` is found, set
  `build.BaseBuildNumber = base.Number` next to `build.BaseBuildID = base.ID`.

This is additive and backward compatible (older builds have `0` → frontend shows no
per-file delta, only current coverage).

## Data flow

```
/report/:scm/:ns/:name
  → GET /api/v1/repos/:scm/:ns/:name              (repo header; 401 → sign-in notice)
  → GET .../builds                                 (BuildList)
/report/:scm/:ns/:name/builds/:number
  → GET .../builds/:number                         (current: sourceFiles+jobs)
  → if baseBuildNumber>0: GET .../builds/:base     (base: sourceFiles) → per-file Δ
/report/:scm/:ns/:name/builds/:number/source/:path+
  → ensure build loaded (commit + file coverage)
  → GET .../content/:path?gitref=<commit>          (401 → sign-in prompt)
```

## Error handling

- Build list/detail 401 (private repo, not authed) → inline "sign in to view this
  repository" with a login link; no crash.
- Build/build-number not found (404) → "build not found" empty state.
- Source content 401 → "sign in to view source" card (coverage UI still shown).
- Source content 404/500 → notice; coverage coloring not shown.
- Missing `sourceFiles` (list rows, or processing builds) → coverage shown as pending/empty.

## Testing

- `lib/coverage.ts`: table-driven unit tests (ratios incl. all-null → 0; line states).
- `stores/build.ts`: `fetchList`, `fetchBuild` (+ base fetch when `baseBuildNumber>0`),
  `fetchSource` (success + 401) with mocked Axios.
- `BuildList.vue`: renders rows, branch filter narrows the list, PR vs branch label.
- `FileCoverageTable.vue`: computes per-file % and Δ from current+base props.
- `SourceLines.vue`: hit/miss/none classes + hit-count gutter for a small fixture.
- `SourceView.vue`: 401 path shows the sign-in prompt.
- Backend: extend `modules/build` finalize test to assert `BaseBuildNumber` is set to the
  base build's number; build store round-trip persists `BaseBuildNumber`.

## Out of scope (B3 / later)

- Folder/tree file view; branch- and PR-dedicated views; parallel-jobs/flags detail UI.
- Virtualized rendering for very large source files.
- Repo settings, badge/card embed UI, coveralls upload guide (B3).
- Removing the legacy `Report` read endpoints (once nothing references them).
