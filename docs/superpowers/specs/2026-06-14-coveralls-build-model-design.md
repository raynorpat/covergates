# Sub-project A — Coveralls-compatible Build/Job Model & Upload

**Date:** 2026-06-14
**Status:** Approved design, ready for implementation planning
**Part of:** "Make covergates work like coveralls.io" (Sub-project A of A/B/C)

## Background

Covergates is a self-hosted coverage service (Go backend, Vue 2/Vuetify frontend).
The goal is to make it behave like [coveralls.io](https://coveralls.io): its build/job
workflow, its site UI, its CI upload integration, and its notifications. That total
scope is decomposed into three sub-projects, built in dependency order:

| # | Sub-project | Depends on |
|---|---|---|
| **A** | Build/Job model + coveralls-compatible upload (this doc) | — |
| B | Coveralls-style site UI | A |
| C | Notifications & checks | A |

### Current model vs. coveralls

Covergates today stores a `Report` keyed by `(ReportID, Commit)`. Uploading to the same
commit **upserts** (overwrites). There is no build number, no concept of separate "jobs"
that merge, and no sequential per-repo history. The server parses raw language reports
(Go/Python/Ruby/lcov/clover/perl) into coverage.

Coveralls instead models a repo as a sequence of **Builds** (each with a per-repo number,
tied to a commit + branch + optional PR). A build is composed of one or more **Jobs**
(parallel CI jobs) merged into one combined coverage set, finalized by a "build done"
signal. Each build computes a **coverage delta** against a base build. Ingestion is via a
universal `POST /api/v1/jobs` endpoint where the **client/reporter** sends already-parsed
per-line `source_files`, plus `POST /webhook` to finalize parallel builds.

## Decisions (from brainstorming)

1. **Exact coveralls wire compatibility** — official coveralls reporters and the GitHub
   Action work unmodified (pointed at this server via `COVERALLS_ENDPOINT`).
2. **Coveralls-only ingestion** — drop server-side language parsers and native upload.
   Coverage line data is produced client-side by the reporter.
3. **Separate secret `repo_token`** — distinct from public addressing (`scm/namespace/name`).
4. **Build identity:** group jobs by CI `service_number`; fall back to `commit` when absent.
   Also assign a per-repo monotonic `Number` for clean ordering/display.
5. **Delta base:** push builds vs. last build on the same branch; PR builds vs. last build
   on the PR's base branch.
6. **In scope for A:** parallel jobs + done webhook. **Deferred:** flags (`flag_name`,
   accepted-but-ignored), carryforward of unchanged files.
7. **A→B transition:** keep legacy `Report` tables + existing frontend read paths alive
   during A so `master` is never broken; B migrates the UI onto `Build`.
8. **CLI:** remove the `covergates` CLI binary entirely.

## Architecture

```
Repo (existing) ──< Build ──< Job
                      │
                      └── merged CoverageData (JSON blob of source files)
```

### Data model (new GORM tables)

**`Build`** — one CI build, belongs to a Repo:

| Field | Notes |
|---|---|
| `RepoID` | FK, indexed |
| `Number` | per-repo monotonic display number, assigned on create |
| `ServiceNumber` | CI's build number; unique with `RepoID`; used for job grouping |
| `ServiceName` | e.g. `github-actions` |
| `Commit` | head SHA, indexed |
| `Branch` | |
| `PullRequest` | int, 0 = none |
| `Status` | `processing` \| `done` \| `errored` |
| `Parallel` | bool |
| `Coverage` | float64, aggregate statement coverage |
| `CoverageChange` | float64, delta vs base build |
| `BaseBuildID` | nullable FK to the build delta was computed against |
| `CommitMessage`, `AuthorName`, `AuthorEmail` | from `git.head` for display |
| `CoverageData` | `[]byte` — merged per-file line coverage (JSON); same blob pattern as existing `Coverage.Data` |
| timestamps + `FinishedAt` | |

**`Job`** — one parallel job under a build:

| Field | Notes |
|---|---|
| `BuildID` | FK, indexed |
| `ServiceJobID`, `ServiceJobNumber` | from payload |
| `Coverage` | this job's coverage |
| `SourceFilesData` | `[]byte` — raw uploaded `source_files`, kept for re-merge/audit |

New core types: `core.Build`, `core.Job`, `core.SourceFile`
(`{Name string, SourceDigest string, Coverage []*int}` — `nil` element = non-relevant line).

`core.SourceFile.Coverage` (`[]*int`) maps to/from existing `core.File.StatementHits`
(`{LineNumber, Hits}`): array index `i` → line `i+1`; `nil` = no statement, otherwise hit count.

Legacy `Report`/`Coverage`/`Reference`/`ReportComment` tables and stores are **unchanged**
and remain registered in `models.tables` for the duration of A.

### Repo token model

- Add `Repo.Token` (random, unique, indexed secret), generated on repo activation.
- Existing repos: backfilled with a generated token on migration (or lazily on first
  settings view). Spec assumes a one-time backfill in `Migrate()`.
- Uploads authenticate via payload `repo_token` → `Repo.Token`.
- Add rotate endpoint `PATCH /api/v1/repos/:scm/:namespace/:name/token`, mirroring the
  existing `HandleReportIDRenew`.
- Public pages keep using `scm/namespace/name`; `ReportID` remains the legacy public badge
  slug for now (removed in B if appropriate).

### Upload — `POST /api/v1/jobs`

Accepts coveralls' multipart form (`json_file` part) **and** a raw JSON body.

Parsed fields: `repo_token`, `service_name`, `service_number`, `service_job_id`,
`service_job_number`, `service_pull_request`, `parallel`, `flag_name` (ignored),
`git.head.{id,message,author_name,author_email}`, `git.branch`, `source_files[]`.

Flow:
1. Resolve `Repo` by `repo_token`; unknown → `422 {"message": ...}`.
2. Find-or-create `Build` by `(RepoID, ServiceNumber)` (fallback `(RepoID, Commit)`),
   assigning the next per-repo `Number` on create. Populate commit/branch/PR/git head.
3. Create a `Job` with this payload's `source_files` and computed job coverage.
4. If **not** `parallel`: finalize the build now (single-job merge — see below).
   If `parallel`: leave `Status = processing`.
5. Respond coveralls-style: `200 {"id": <jobID>, "url": "<server>/report/...", "message": "Job created"}`.

### Parallel finalize — `POST /webhook?repo_token=...`

Registered at the **engine root** (not under `/api/v1`), because coveralls reporters append
`/webhook` to the endpoint base.

Body: `{"payload": {"build_num": "<service_number>", "status": "done"}}`.
Resolve repo by `repo_token`, find the open build by `(RepoID, ServiceNumber=build_num)`,
finalize, return `{"done": true}`. Unknown build / bad token → `422`.

Finalization is a clean, single hook point (`BuildStore.Finalize` + a `finalizeBuild`
service step) so Sub-project C can later attach PR comments / status checks. A only
computes and stores.

### Merge algorithm (pure function, unit-tested standalone)

Given N jobs' `source_files`:
- Union files by `name`.
- For each file, for each line index: `nil` if every job has `nil` there; otherwise the
  sum of non-`nil` hit counts.
- Build coverage % = covered relevant lines (merged hits > 0) ÷ total relevant lines
  (merged value non-`nil`) across all files.

The merged result is serialized into `Build.CoverageData`.

### Delta / base build selection (on finalize)

- If `PullRequest > 0`: base = latest `done` build on the PR's **base branch**, resolved
  via the existing SCM PR lookup (`modules/scm` PR API) using the repo creator's creds;
  fall back to the repo's default branch if unavailable.
- Else: base = the most recent earlier `done` build on the **same branch**.
- `CoverageChange = Build.Coverage − base.Coverage`; persist `BaseBuildID`.

### Store + read endpoints

New `core.BuildStore` interface (with mockgen mock):

```
Create(*Build) error
AddJob(*Build, *Job) error
FindByServiceNumber(repoID uint, serviceNumber string) (*Build, error)
FindByNumber(repoID uint, number int) (*Build, error)
FindByCommit(repoID uint, commit string) (*Build, error)
ListByRepo(repoID uint, branch string, limit, offset int) ([]*Build, error)
LatestOnBranch(repoID uint, branch string) (*Build, error)
Finalize(*Build) error
```

Minimal read endpoints (so A is verifiable and B has a foundation):
- `GET /api/v1/repos/:scm/:namespace/:name/builds` — list builds (paginated).
- `GET /api/v1/repos/:scm/:namespace/:name/builds/:number` — build detail w/ merged files.
- Badge endpoint repointed to the latest default-branch `done` build's coverage.

### What gets removed

- Native `POST /api/v1/reports/:id` upload handler (`report.HandleUpload`).
- `core.CoverageService` and the parser packages: `service/golang`, `service/python`,
  `service/ruby`, `service/lcov`, `service/clover`, `service/perl`, `service/coverage`
  (and `service/common` if unused after).
- The `covergates` CLI binary: delete `cmd/cli/` entirely.
- Wiring updates: remove `CoverageService` from `wire`/`inject_*`, the API `Router`,
  and route registration.
- **Build/CI updates:** remove CLI build steps from `Dockerfile` and
  `.github/workflows/{ci,cd}.yml` (these recently added CLI builds).

The remaining `ReportStore` / `ReportService` / chart endpoints stay compiling so the
legacy site survives the A→B gap.

## Data flow

```
CI reporter ──POST /api/v1/jobs (json_file)──▶ resolve repo by token
                                              │
                                              ▼
                                   find/create Build (+Number)
                                              │
                                              ▼
                                      create Job (source_files)
                                              │
                        parallel? ── no ──▶ finalize build now
                            │
                           yes
                            ▼
CI reporter ──POST /webhook (build_num=done)──▶ finalize build
                                              │
                                              ▼
                          merge jobs → Build.CoverageData + Coverage
                                              │
                                              ▼
                          select base build → CoverageChange
                                              │
                                              ▼
                              Status=done   (hook point for C)
```

## Error handling

| Case | Response |
|---|---|
| Missing/invalid `repo_token` | `422 {"message": "..."}` |
| Malformed payload / no `source_files` | `422 {"message": "..."}` |
| `/webhook` unknown build or bad token | `422` |
| Merge/finalize failure | `500`, build `Status=errored` |

Reporters treat any non-2xx as failure; success bodies include `id`/`url`/`message`.

## Testing

- **Pure logic** (merge, delta selection, build numbering, source-file ↔ StatementHit
  conversion): table-driven unit tests.
- **Handlers** (`/jobs` single, `/jobs` parallel, bad token, `/webhook` done, unknown
  build): handler tests following existing `routers/api/report/report_test.go` + mock
  patterns.
- **Store**: sqlite-backed tests via the existing `models/tests` harness.

## Out of scope (later sub-projects / deferred)

- Site UI redesign (Sub-project B).
- PR comments, commit status checks, decrease alerts, email/Slack (Sub-project C).
- Flags (`flag_name`) grouping; carryforward of unchanged files.
- Removing legacy `Report` model/tables (handled in B once the UI no longer reads them).
