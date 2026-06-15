# Coveralls Notifications & Checks — C1 (Verdict + Dispatch + Commit Status) Design

**Date:** 2026-06-15
**Sub-project:** C (notifications & checks), chunk **C1** of 3.
**Branch:** `feat/coveralls-notify-c1`

## Goal

After a build finalizes, compute a single pass/fail/error **coverage verdict** from the
build, its base build, and the repo's policy settings, then post a **commit status**
(`coverage/covergates`) to the build's commit on the SCM. This is coveralls' core
"check" — it appears in the GitHub/Gitea PR merge box and can gate merges.

C1 establishes the dispatch architecture (a `NotifyService` that fans out to `Notifier`s)
that C2 (PR comments) and C3 (email/Slack) plug into.

## Context

- Builds finalize in `modules/build/service.go` `Finalize`, called from
  `routers/api/build/build.go` `HandleJobs` (non-parallel) and `HandleWebhook` (parallel).
  After finalize, `build.Coverage`, `build.CoverageChange`, and `build.BaseBuildID` are set.
- The SCM `core.Client` exposes `Repositories() GitRepoService` but has **no** commit-status
  method today. The underlying `github.com/drone/go-scm` (`v1.27.0`) provides
  `Repositories.CreateStatus(ctx, repo, ref, *scm.StatusInput)` with states
  `StatePending/StateSuccess/StateFailure/StateError`.
- PR/SCM operations already reuse the repo creator's token via `s.Repos.Creator(repo)`
  (see `Service.pullRequestBase` in `modules/build/service.go`).
- `core.RepoSetting` is persisted as a JSON blob (`models.RepoSetting.Config`), so new
  setting fields need **no migration**.
- The settings UI lives in `web/src/components/SettingsGeneral.vue` and
  `web/src/views/SettingsView.vue`, with the repo setting type in `web/src/types`.
- Mocks are hand-written under `mock/` (e.g. `mock/scm_mock.go`).
- Server base URL is available via `config.Config` (`cfg.Server.URL()`).

## Decisions (from brainstorming)

1. **Post identity:** repo creator's SCM token (reuse `RepoStore.Creator`). No new config.
2. **Pass/fail policy:** decrease threshold **and** absolute minimum (two per-repo knobs).
3. **Dispatch architecture:** a separate `NotifyService` (module `modules/notify`) called
   right after `Finalize` by the build handlers. Notifications are best-effort and never
   fail the build.
4. **Scope:** C1 = verdict + dispatch + commit status only. PR comments (C2) and
   email/Slack (C3) are later chunks.

## Architecture

```
HandleJobs / HandleWebhook
  └─ buildService.Finalize(ctx, repo, build)   # sets Coverage, Change, BaseBuildID, Status
  └─ notifyService.Notify(ctx, repo, build)    # NEW — best-effort
        ├─ setting = Repos.Setting(repo)
        ├─ verdict = Evaluate(build, setting)   # pure
        └─ for each notifier: notifier.Notify(ctx, repo, build, verdict)  # log errors
              └─ StatusNotifier → client.Repositories().CreateStatus(...)
```

### Component 1 — core types (`core/notify.go`, new)

```go
package core

import "context"

//go:generate mockgen -package mock -destination ../mock/notify_mock.go . NotifyService,Notifier

// StatusState is a commit-status / verdict state.
type StatusState string

const (
	StatusPending StatusState = "pending"
	StatusSuccess StatusState = "success"
	StatusFailure StatusState = "failure"
	StatusError   StatusState = "error"
)

// Status is a commit status to post to an SCM.
type Status struct {
	State  StatusState
	Label  string // status context, e.g. "coverage/covergates"
	Desc   string // short human description
	Target string // URL the status links to (build detail page)
}

// Verdict is the evaluated outcome of a finalized build under the repo's policy.
type Verdict struct {
	State       StatusState
	Coverage    float64 // ratio 0..1
	Change      float64 // ratio delta vs base (negative = decrease)
	HasBase     bool
	Description string
}

// Notifier delivers a build's verdict over one channel (status, comment, email, ...).
type Notifier interface {
	Notify(ctx context.Context, repo *Repo, build *Build, verdict *Verdict) error
}

// NotifyService computes the verdict for a finalized build and dispatches notifiers.
type NotifyService interface {
	Notify(ctx context.Context, repo *Repo, build *Build) error
}
```

### Component 2 — SCM commit status

Add one method to `core.GitRepoService` (in `core/scm.go`):

```go
CreateStatus(ctx context.Context, user *User, repo, ref string, status *Status) error
```

Implement in `modules/scm/repo.go` on the existing `repoService`:

```go
func (service *repoService) CreateStatus(ctx context.Context, user *core.User, repo, ref string, status *core.Status) error {
	ctx = withUser(ctx, service.scm, user)
	input := &scm.StatusInput{
		State:  toSCMState(status.State),
		Label:  status.Label,
		Title:  status.Label,
		Desc:   status.Desc,
		Target: status.Target,
	}
	_, _, err := service.client.Repositories.CreateStatus(ctx, repo, ref, input)
	return err
}

func toSCMState(s core.StatusState) scm.State {
	switch s {
	case core.StatusSuccess:
		return scm.StateSuccess
	case core.StatusFailure:
		return scm.StateFailure
	case core.StatusError:
		return scm.StateError
	default:
		return scm.StatePending
	}
}
```

Add `CreateStatus` to the hand-written `mock/scm_mock.go` `MockGitRepoService`.

### Component 3 — verdict policy (`modules/notify/verdict.go`, new)

Two new fields on `core.RepoSetting` (percent units; `0` disables that check):

```go
CoverageMinimum           float64 `json:"coverageMinimum"`
CoverageDecreaseThreshold float64 `json:"coverageDecreaseThreshold"`
```

`Evaluate` (pure):

```go
func Evaluate(build *core.Build, setting *core.RepoSetting) *core.Verdict {
	v := &core.Verdict{
		Coverage: build.Coverage,
		Change:   build.CoverageChange,
		HasBase:  build.BaseBuildID > 0,
	}
	covPct := build.Coverage * 100

	if build.Status == core.BuildErrored {
		v.State = core.StatusError
		v.Description = "Coverage build failed"
		return v
	}

	v.State = core.StatusSuccess
	if setting != nil && setting.CoverageMinimum > 0 && covPct < setting.CoverageMinimum {
		v.State = core.StatusFailure
		v.Description = fmt.Sprintf("Coverage %.1f%% is below the minimum %.1f%%", covPct, setting.CoverageMinimum)
		return v
	}
	if v.HasBase {
		decreasePct := -build.CoverageChange * 100 // positive when coverage dropped
		threshold := 0.0
		if setting != nil {
			threshold = setting.CoverageDecreaseThreshold
		}
		if decreasePct > threshold {
			v.State = core.StatusFailure
			v.Description = fmt.Sprintf("Coverage decreased %.1f%% to %.1f%%", decreasePct, covPct)
			return v
		}
	}
	v.Description = describePass(covPct, build.CoverageChange, v.HasBase)
	return v
}
```

`describePass` → `"Coverage: 85.3% (+0.5%)"` when there is a base, else `"Coverage: 85.3%"`.
Sign is explicit (`+`/`-`); `0.0%` change renders as `(+0.0%)`.

### Component 4 — dispatch (`modules/notify/service.go`, new)

```go
type Service struct {
	Repos     core.RepoStore
	Notifiers []core.Notifier
}

func (s *Service) Notify(ctx context.Context, repo *core.Repo, build *core.Build) error {
	setting, err := s.Repos.Setting(repo)
	if err != nil {
		setting = &core.RepoSetting{}
	}
	verdict := Evaluate(build, setting)
	for _, n := range s.Notifiers {
		if err := n.Notify(ctx, repo, build, verdict); err != nil {
			log.Errorf("notifier failed: %v", err) // sirupsen/logrus, as used elsewhere
		}
	}
	return nil
}
```

Best-effort: a notifier error is logged, never returned. `Notify` always returns `nil`
in C1 (the build handlers ignore its return but call it for effect).

### Component 5 — commit-status notifier (`modules/notify/status.go`, new)

```go
type StatusNotifier struct {
	SCM    core.SCMService
	Repos  core.RepoStore
	Config *config.Config
}

func (n *StatusNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	if build.Commit == "" {
		return nil
	}
	user, err := n.Repos.Creator(repo)
	if err != nil {
		return err
	}
	client, err := n.SCM.Client(repo.SCM)
	if err != nil {
		return err
	}
	status := &core.Status{
		State:  verdict.State,
		Label:  "coverage/covergates",
		Desc:   verdict.Description,
		Target: buildURL(n.Config, repo, build),
	}
	return client.Repositories().CreateStatus(ctx, user, repo.FullName(), build.Commit, status)
}

func buildURL(cfg *config.Config, repo *core.Repo, build *core.Build) string {
	return fmt.Sprintf("%s/report/%s/%s/builds/%d", cfg.Server.URL(), repo.SCM, repo.FullName(), build.Number)
}
```

### Component 6 — wiring

- `config/wire_gen.go` (hand-edited): construct `*notify.Service` with
  `Notifiers: []core.Notifier{ &notify.StatusNotifier{SCM, Repos, Config} }`, and pass it
  to the build route registration.
- `routers/api/build/build.go`: add a `core.NotifyService` parameter to `HandleJobs` and
  `HandleWebhook`; after a successful `Finalize`, call
  `notifyService.Notify(c.Request.Context(), repo, build)`. (HandleJobs only finalizes when
  `!payload.Parallel`; call Notify in that same branch. HandleWebhook always finalizes.)
- `routers/api/api.go` (or wherever build routes register): thread the `NotifyService`
  through.

### Component 7 — settings UI

In `web/src/types` add `coverageMinimum: number` and `coverageDecreaseThreshold: number`
to the repo setting type. In `SettingsGeneral.vue` (or a new card within `SettingsView.vue`)
add a **Checks** section with two numeric fields:
- "Minimum coverage %" → `coverageMinimum` (0 = no minimum)
- "Max coverage decrease %" → `coverageDecreaseThreshold` (0 = any decrease fails)

Saved through the existing setting PATCH flow (no new endpoint).

## Data flow

1. CI uploads jobs; build finalizes → `Coverage`, `CoverageChange`, `BaseBuildID`, `Status` set.
2. Handler calls `notifyService.Notify(ctx, repo, build)`.
3. Dispatch loads `RepoSetting`, computes `Verdict` via `Evaluate`.
4. `StatusNotifier` resolves the creator token and posts the commit status to `build.Commit`,
   linking to the build detail page.
5. Any notifier error is logged; the upload/webhook response is unaffected.

## Error handling

- Notifier failures (no creator, SCM client error, API error) are logged and swallowed.
- Errored builds produce a `StatusError` commit status ("Coverage build failed").
- Builds with no commit SHA skip the status notifier (return nil).
- A missing/blank `RepoSetting` is treated as all-checks-disabled (build passes unless a
  base decrease with default threshold 0 applies).

## Testing

- `modules/notify/verdict_test.go` — table-driven: errored build; no base + no minimum
  (pass, informational); base with no decrease (pass); decrease within threshold (pass);
  decrease beyond threshold (fail); below minimum (fail); minimum and decrease both set.
  Assert state and description substrings.
- `modules/notify/status_test.go` — uses `mock.MockSCMService`/`MockClient`/
  `MockGitRepoService` + `mock.MockRepoStore`: asserts `CreateStatus` is called with the
  expected ref (`build.Commit`), label `coverage/covergates`, state, and target URL; and
  that a blank commit skips the call.
- `modules/notify/service_test.go` — dispatch computes the verdict and calls each notifier;
  a notifier returning an error is swallowed (Notify returns nil, other notifiers still run).
- `modules/scm` — `CreateStatus` state mapping (`toSCMState`) unit test.
- `models` setting round-trip includes the new fields (JSON blob; runs under CGO only —
  document the local CGO/sqlite limitation).
- `web` — a small Vitest test that the Checks card renders and emits save with edited values.

## Out of scope (later chunks)

- PR comment posting and comment-ID tracking → **C2**.
- Email (server SMTP) and Slack webhook delivery, per-repo recipients/webhook, trigger
  setting → **C3**.
- Pending status posted at build *start* (we only post once, after finalize).
- Retry/queue for failed notifications.
