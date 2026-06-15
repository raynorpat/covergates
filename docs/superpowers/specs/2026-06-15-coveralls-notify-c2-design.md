# Coveralls Notifications & Checks — C2 (PR Coverage Comment) Design

**Date:** 2026-06-15
**Sub-project:** C (notifications & checks), chunk **C2** of 3.
**Branch:** `feat/coveralls-notify-c2`

## Goal

After a build that belongs to a pull request finalizes, post (or refresh) a single
markdown coverage comment on that PR: a pass/fail summary with coverage and delta, plus a
table of the PR's changed files and their coverage. Coveralls-style PR feedback.

## Context

- C1 established the notification architecture: `modules/notify.Service` (best-effort
  dispatcher) computes a `core.Verdict` and fans out to `[]core.Notifier`. C1 added one
  notifier (`StatusNotifier`). C2 adds a second (`CommentNotifier`) to the same slice — no
  handler or DI wiring beyond appending it in `provideNotifyService`.
- `core.Notifier.Notify(ctx, repo, build, verdict)` is the interface. The dispatcher does
  NOT pass the `RepoSetting` to notifiers, so the comment notifier reads its own enable flag
  via `RepoStore.Setting(repo)` (keeps the C1 interface and `StatusNotifier`/mock untouched).
- `modules/build.Service.Finalize` sets `build.SourceFiles` (merged per-file coverage) on
  the build object before the handler dispatches notifications, so the build passed to
  `Notify` carries per-file coverage.
- The SCM `core.PullRequestService` exposes `Find`, `CreateComment(...) (int, error)`,
  `RemoveComment(..., id)`, and `ListChanges(...) ([]*core.FileChange, error)`.
  **go-scm (v1.27.0) has no comment edit** — only create/delete. So updating the comment
  across builds means tracking the previous comment ID, deleting it, and creating a fresh
  one (a single rolling comment per PR).
- The repo creator's SCM token is used for all SCM writes (same `RepoStore.Creator` pattern
  as `StatusNotifier` and the existing PR base lookup).
- `core.RepoSetting` is a JSON blob (`models.RepoSetting.Config`); new fields need no
  migration. The `RepoHook` model + `RepoStore.FindHook/UpdateHook` are the precedent for a
  per-repo auxiliary side record persisted alongside the repo.
- `core.FileChange{Path, Added, Renamed, Deleted}` (JSON-tagged) is what `ListChanges`
  returns. `core.SourceFile{Name, SourceDigest, Coverage []*int}` holds per-line coverage; a
  nil element means a non-executable line.
- C1's build-detail URL format: `<cfg.Server.URL()>/report/<scm>/<fullname>/builds/<number>`.

## Decisions (from brainstorming)

1. **Comment content:** summary (pass/fail + coverage + delta + base ref + build link) **plus**
   a changed-files coverage table (PR changed files × build per-file coverage).
2. **Enablement:** per-repo toggle, **default ON**. Implemented as an inverted flag
   `DisablePRComment bool` (zero-value false ⇒ enabled) so existing saved settings and new
   repos default on with no migration. UI shows a normal "Comment on pull requests" switch.
3. **Update mechanism:** single rolling comment per PR — track last comment ID, delete it,
   create a fresh comment, store the new ID. (go-scm has no edit.)
4. **Best-effort:** the dispatcher logs and swallows notifier errors; comment failures never
   affect coverage uploads. A failed `RemoveComment` (comment already gone) is ignored.

## Architecture

```
notify.Service.Notify(ctx, repo, build)         # C1 dispatcher, after Finalize
  verdict = Evaluate(build, setting)
  for n in Notifiers:
    n.Notify(ctx, repo, build, verdict)
      ├─ StatusNotifier   (C1)
      └─ CommentNotifier  (C2, NEW)
            skip if build.PullRequest == 0
            skip if Repos.Setting(repo).DisablePRComment
            user   = Repos.Creator(repo)
            client = SCM.Client(repo.SCM)
            changes = client.PullRequests().ListChanges(ctx, user, repo.FullName(), build.PullRequest)
            body    = commentBody(repo, build, verdict, changes, coverageByPath(build))
            if prev := Repos.FindPullRequestComment(repo.ID, build.PullRequest); prev > 0:
                _ = client.PullRequests().RemoveComment(ctx, user, repo.FullName(), build.PullRequest, prev)  # ignore err
            id, err := client.PullRequests().CreateComment(ctx, user, repo.FullName(), build.PullRequest, body)
            if err != nil: return err
            return Repos.UpdatePullRequestComment(repo.ID, build.PullRequest, id)
```

### Component 1 — persisted comment record + RepoStore methods

`core/repo.go` — add to the `RepoStore` interface:

```go
// FindPullRequestComment returns the tracked SCM comment ID for a repo's PR,
// or 0 (with nil error) when none has been recorded.
FindPullRequestComment(repoID uint, number int) (int, error)
// UpdatePullRequestComment records the SCM comment ID for a repo's PR.
UpdatePullRequestComment(repoID uint, number, commentID int) error
```

`models/repo.go` — new model + impl:

```go
// PullRequestComment tracks the latest coverage comment posted to a PR.
type PullRequestComment struct {
	gorm.Model
	RepoID    uint `gorm:"index:idx_pr_comment,unique"`
	Number    int  `gorm:"index:idx_pr_comment,unique"`
	CommentID int
}
```
`FindPullRequestComment`: `First` by `{RepoID, Number}`; on `gorm.ErrRecordNotFound` return `(0, nil)`; other errors propagate. `UpdatePullRequestComment`: `FirstOrCreate` by `{RepoID, Number}`, set `CommentID`, `Save`.

`models/models.go` — add `&PullRequestComment{}` to the `Migrate()` tables list.

### Component 2 — settings field

`core/repo.go` — add to `RepoSetting`:

```go
// DisablePRComment turns off the coverage comment posted to pull requests.
// Zero value (false) means comments are enabled.
DisablePRComment bool `json:"disablePRComment"`
```

### Component 3 — markdown body + per-file coverage (`modules/notify/comment.go`)

Pure helpers (no I/O), independently tested:

```go
// coverageByPath maps each build source file to its coverage ratio in [0,1],
// or -1 when the file has no relevant (executable) lines.
func coverageByPath(build *core.Build) map[string]float64

// commentBody renders the PR coverage comment markdown.
func commentBody(repo *core.Repo, build *core.Build, verdict *core.Verdict,
	changes []*core.FileChange, cov map[string]float64, target string) string
```

`coverageByPath`: for each `SourceFile`, count non-nil coverage entries as relevant and
entries `> 0` as covered; ratio = covered/relevant, or `-1` if relevant == 0.

`commentBody` format:

```
## Coverage <emoji> <Title>
<verdict.Description>

[Build #<Number>](<target>)[ · base #<BaseBuildNumber>]

### Changed files
| File | Coverage |
| --- | --- |
| <path> | <pct or —> |
```

- Emoji/title by `verdict.State`: success → `✅ passed`; failure → `❌ failed`;
  error → `⚠️ errored`; (pending unused here).
- Base ref ` · base #M` only when `build.BaseBuildNumber > 0`.
- Changed-files section only when `len(changes) > 0`. Each changed file's cell:
  deleted files → `deleted`; a path present in `cov` with ratio ≥ 0 → `NN.N%`; otherwise `—`.
- Coverage percent uses one decimal (`%.1f%%`), consistent with C1 verdict descriptions.

### Component 4 — CommentNotifier (`modules/notify/comment.go`)

```go
type CommentNotifier struct {
	SCM    core.SCMService
	Repos  core.RepoStore
	Config *config.Config
}
func (n *CommentNotifier) Notify(ctx, repo, build, verdict) error
```
Logic exactly as the architecture diagram. Uses the C1 build-URL format for `target`
(extract the shared helper or duplicate the one-line `fmt.Sprintf`; duplication is
acceptable and avoids coupling the two notifier files).

### Component 5 — wiring

`cmd/server/inject_service.go` — append to the `Notifiers` slice in `provideNotifyService`:

```go
Notifiers: []core.Notifier{
	&notifymod.StatusNotifier{SCM: scmService, Repos: repoStore, Config: config},
	&notifymod.CommentNotifier{SCM: scmService, Repos: repoStore, Config: config},
},
```
No other wiring changes (handlers, routers, wire_gen call graph unchanged — same deps).

### Component 6 — mocks

- `mock/scm_mock.go` — add `MockPullRequestService` (gomock style: `Find`, `CreateComment`,
  `RemoveComment`, `ListChanges`); add `PullRequestService` to the `//go:generate` list in
  `core/scm.go`.
- `mock/repo_mock.go` — add `FindPullRequestComment` and `UpdatePullRequestComment` to
  `MockRepoStore`.

### Component 7 — frontend toggle

- `web/src/types/setting.ts` — add `disablePRComment: boolean`.
- `web/src/components/SettingsChecks.vue` — add a `v-switch` "Comment on pull requests"
  bound to `!disablePRComment`; on save emit `disablePRComment: !switchValue`. Existing
  fixtures that construct `RepoSetting` literals must add the field.

## Data flow

1. CI uploads jobs → build finalizes (`SourceFiles`, `Coverage`, `CoverageChange`,
   `BaseBuildNumber`, `PullRequest` set).
2. Handler calls `notifyService.Notify(ctx, repo, build)`.
3. Dispatcher computes verdict, calls each notifier.
4. `CommentNotifier`: PR build + enabled → fetch changed files, render body, delete prior
   comment (if any), create new comment, persist its ID.
5. Errors logged best-effort; the HTTP upload/webhook response is unaffected.

## Error handling

- Non-PR build or `DisablePRComment` → return nil (no-op).
- `Creator`/`Client`/`CreateComment` errors → returned (dispatcher logs them).
- `ListChanges` error → returned (don't post a partial comment). The dispatcher logs it;
  the build/status are unaffected.
- `RemoveComment` error (stale/deleted prior comment) → ignored; proceed to create.
- `FindPullRequestComment` returns `(0, nil)` when no prior comment; a real store error is
  returned.

## Testing

- `modules/notify/comment_body_test.go` — `coverageByPath` (covered/relevant/no-relevant →
  -1) and `commentBody`: pass/fail/error header+emoji; base ref present/absent; changed-files
  table with covered file, no-coverage file (`—`), deleted file (`deleted`); no-changes case
  omits the table; percent formatting.
- `modules/notify/comment_test.go` — `CommentNotifier` via `MockSCMService`/`MockClient`/
  `MockPullRequestService`/`MockRepoStore`:
  - non-PR build (`PullRequest == 0`) → no SCM calls, nil.
  - disabled (`DisablePRComment == true`) → reads setting, no SCM calls, nil.
  - first comment (FindPullRequestComment → 0): ListChanges, CreateComment, then
    UpdatePullRequestComment with the returned id; no RemoveComment.
  - subsequent comment (FindPullRequestComment → 42): RemoveComment(42) then CreateComment +
    UpdatePullRequestComment with the new id.
  - RemoveComment error is swallowed (still creates + persists).
- `models` round-trip for `PullRequestComment` (find-missing → 0; update then find) —
  documented as CGO/sqlite-only (can't run in this environment).
- `web` — `SettingsChecks.spec.ts` updated to drive the new switch and assert
  `disablePRComment` in the emitted payload; existing `RepoSetting` fixtures get the field.

## Out of scope (later chunk)

- Email (server SMTP) and Slack delivery, per-repo recipients/webhook, trigger setting → **C3**.
- Comment editing (go-scm has no edit; we delete+recreate).
- Inline per-line review comments on the diff (only a single summary comment).
- Posting a comment when a build has no PR, or on push/branch builds.
