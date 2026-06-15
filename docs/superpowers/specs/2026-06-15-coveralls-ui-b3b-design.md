# Sub-project B3b — Coverage Browsing Polish (tree, branches, jobs/flags, PR files)

**Date:** 2026-06-15
**Status:** Approved design, ready for implementation planning
**Part of:** "Make covergates work like coveralls.io" — Sub-project B (UI), chunk **B3b** (final B chunk)

## Background

A + B1 + B2 + B3a are merged into `shuenhoy/master` and pushed. B3b finishes the
coveralls-style browsing UI: a folder/tree file view, a branches overview, a parallel-jobs
panel with flag support (incl. the backend flag storage A deferred), and a PR changed-files
view. After B3b, Sub-project B is complete; C (notifications & checks) remains.

### Relevant existing API / code

- `GET /api/v1/repos/:scm/:namespace/:name/builds` → `Build[]` (public).
- `GET .../builds/:number` → `Build` + merged `sourceFiles` + `jobs` (jobs have id,
  serviceJobID, coverage; `SourceFiles` nulled on the response).
- Build detail (B2) renders summary + per-file flat table (`FileCoverageTable`) + per-file
  delta vs base build.
- `routers/api/build/payload.go` parses `flag_name` into `coverallsPayload.FlagName` but
  `HandleJobs` does not persist it; `core.Job`/`models.Job` have no flag field.
- `core.PullRequestService.ListChanges(ctx, user, repo, number) ([]*core.FileChange, error)`;
  `core.FileChange { Path string; Added, Renamed, Deleted bool }`.
- The authenticated repo route group (`g.Use(checkLogin)`) hosts `/content/*path`, `/setting`,
  etc.; the public group hosts `/builds`, `/builds/:number`.
- Frontend: Vue 3 + Vuetify 3 + Pinia; `build` store (`fetchList`, `fetchBuild`,
  `fetchSource`); `lib/coverage.ts` (`fileCoverage`, `formatPercent`, `coverageColor`);
  `lib/auth.ts` (`is401`, `loginUrl`). Build types in `web/src/types/build.ts`.

## Decisions (from brainstorming)

1. Folder file view: **flat ↔ tree toggle** (keep B2's flat table, add a tree).
2. Build all four: folder tree, branches overview, jobs+flags panel, PR changed-files view.
3. Flags need backend storage (`Job.Flag`); PR changed-files needs a new SCM-backed endpoint.
4. Branches overview is derived client-side from the builds list (no new backend).
5. PR changed-files is a **panel on the build detail** (shown when `pullRequest > 0`), not a
   separate route. Jobs panel is also on the build detail.

## Architecture

### File structure

Backend (modify): `core/build.go` (`Job.Flag`), `models/build.go` (`Job` flag column +
`AddJob`/`toCoreJob`), `routers/api/build/build.go` (`HandleJobs` sets `Flag`; new
`HandleChanges`), `routers/api/api.go` (route), plus tests in `models/build_test.go`,
`routers/api/build/build_test.go`.

Frontend (create): `web/src/lib/fileTree.ts` (+ test), `web/src/components/FileCoverageTree.vue`
(+ test), `web/src/components/JobsPanel.vue` (+ test), `web/src/components/ChangedFilesPanel.vue`
(+ test), `web/src/views/BranchesView.vue` (+ test).
Frontend (modify): `web/src/types/build.ts` (`Job.flag`, `FileChange`), `web/src/stores/build.ts`
(`fetchChanges`), `web/src/views/BuildDetailView.vue` (toggle + jobs panel + PR panel),
`web/src/views/BuildsView.vue` (Branches link), `web/src/router/index.ts` (branches route).

### B3b.1 Backend: job flags

- `core.Job` += `Flag string \`json:"flag"\``.
- `models.Job` += `Flag string` column; `AddJob` sets `m.Flag = j.Flag`; `toCoreJob` maps it.
- `routers/api/build/build.go` `HandleJobs`: set `job.Flag = payload.FlagName` when building
  the `core.Job`.
- Build-detail `jobs` already returned; now each job carries `flag`.

### B3b.2 Backend: PR changed-files endpoint

`GET /api/v1/repos/:scm/:namespace/:name/pulls/:number/changes` →
`200 [] core.FileChange` (`{path, added, renamed, deleted}`). Handler `HandleChanges`:
require a context user (401 if absent, matching `/content`); resolve the SCM client; call
`client.PullRequests().ListChanges(ctx, user, repo.FullName(), number)`; return the slice.
Registered in the **authenticated** repo group. 400 on bad number, 500 on SCM error.

### B3b.3 Folder tree (build detail)

`lib/fileTree.ts` — pure: `buildTree(files: {name, covered, relevant}[]) → TreeNode` where
each directory node aggregates `covered`/`relevant` from descendants and leaf nodes are
files; children sorted folders-first then by name. Unit-tested for nesting + aggregation.

`FileCoverageTree.vue` — renders the tree with Vuetify list/expansion; each row shows name +
coverage % (colored); file rows link to source (`{buildRoute}/source/{path}`); folder rows
show rolled-up %. Per-file Δ is omitted in the tree (the flat table keeps it).

`BuildDetailView.vue` — a `v-btn-toggle` (Flat / Tree). Flat → existing `FileCoverageTable`;
Tree → `FileCoverageTree` (built from `build.sourceFiles` via `fileCoverage` + `buildTree`).

### B3b.4 Branches overview

`BranchesView.vue` at `/report/:scm/:namespace/:name/branches` (public; 401 inline notice):
`build.fetchList(repoPath)`, then group by `branch`, take the highest-`number` build per
branch, and render a table (branch · coverage chip · build # link · status). A "Branches"
text button on `BuildsView` links here. No new backend (derived from the list).

### B3b.5 Jobs + flags panel

`JobsPanel.vue` (on build detail, when `build.jobs?.length`): a table of jobs — flag (or "—")
and per-job coverage (`formatPercent(job.coverage)`). Surfaces parallel jobs and their flags.

### B3b.6 PR changed-files panel

`ChangedFilesPanel.vue` (on build detail, when `build.pullRequest > 0`): calls
`build.fetchChanges(repoPath, number)`; for each non-deleted change, looks up the file in
`build.sourceFiles` by path and shows its coverage (`fileCoverage`), or "no coverage data"
when absent. File rows link to source. 401 → "sign in to view changed files" note (anon
can't diff a PR). Deleted files shown struck-through without coverage.

### B3b.7 Data layer

- `web/src/types/build.ts`: `Job` += `flag: string`; add `FileChange { path: string;
  added: boolean; renamed: boolean; deleted: boolean }`.
- `web/src/stores/build.ts`: `fetchChanges(repoPath, number) → FileChange[]` via
  `GET {repoPath}/pulls/{number}/changes` (returns `data ?? []`).

## Data flow

```
/report/:scm/:ns/:name/builds/:number
  → GET .../builds/:number            (build + sourceFiles + jobs[flag])
  → (PR) GET .../pulls/:number/changes (ChangedFilesPanel; 401 → sign-in)
  Flat/Tree toggle builds the tree from sourceFiles client-side.
/report/:scm/:ns/:name/branches
  → GET .../builds → group by branch → latest per branch
POST /api/v1/jobs (existing) now persists flag_name → Job.Flag
```

## Error handling

- Changes endpoint 401 → ChangedFilesPanel shows a sign-in note; the rest of the build
  detail still renders.
- Changes endpoint 500/SCM error → panel shows an inline error; build detail unaffected.
- Branches/builds 401 → inline notice (reuses B2/B3a pattern).
- Tree build on empty `sourceFiles` → empty tree / "no files" note.

## Testing

- Backend: `models/build_test.go` round-trips `Job.Flag`; `build_test.go` — `HandleJobs`
  persists `flag` (assert via `AddJob` capture), and `HandleChanges` returns changes for an
  authorized user (mock `ListChanges`) + 401 without a user.
- Frontend: `fileTree` (nesting + folder aggregation), `FileCoverageTree` (renders folders +
  files, links), `BranchesView` (latest-per-branch grouping), `JobsPanel` (flag + coverage),
  `ChangedFilesPanel` (changed files w/ coverage + 401 note), build-store `fetchChanges`.

## Out of scope

- Per-flag coverage *merging* into separate build views (jobs panel shows per-job coverage +
  flag; no separate per-flag aggregate build). 
- Carryforward of unchanged files; legacy `Report` removal.
- Sub-project C (notifications & checks).
