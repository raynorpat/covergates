# Coveralls Notifications & Checks — C3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a build finalizes, deliver a coverage summary over email (server SMTP + per-repo recipients) and/or Slack (per-repo webhook), gated by a per-repo trigger (off / on-failure / always).

**Architecture:** Two more `core.Notifier`s (`EmailNotifier`, `SlackNotifier`) appended to the C1 best-effort dispatch slice. Each reads its own config via `RepoStore.Setting`, applies a shared trigger gate, and delivers to its target. Email uses stdlib `net/smtp` (injectable sender for tests); Slack uses stdlib `net/http` (injected client, tested via `httptest`). Shared `buildURL`/`statusLabel`/`summaryLine`/`shouldNotify` helpers consolidate logic (and de-duplicate the build-URL string from C1/C2).

**Tech Stack:** Go 1.14 (stdlib `net/smtp`, `net/http`, `encoding/json`), Gin, GORM, google/wire, gomock; `kelseyhightower/envconfig`; Vue 3 + Vuetify 3 + Vitest frontend. No new dependencies.

---

## Environment notes (read first)

- **No CGO locally:** build/test with `CGO_ENABLED=0` from the **repo root** `D:\raynorpat\covergates`; cwd persists across Bash calls, so never `cd` into a subdir for go commands. Frontend commands run from `web/`.
- **Coverage units:** `core.Build.Coverage`/`CoverageChange` are ratios in `[0,1]`.
- **Mocks are committed** (`mock/*.go`). Only `MockRepoStore` is needed here (already exists; `Setting` already mocked). No new mock types.
- **Builds dispatched to notifiers** already have `Coverage`, `CoverageChange`, `BaseBuildNumber`, `Number`, `Status`, `PullRequest` set by `modules/build.Service.Finalize`.

## Background from C1/C2 (do not break)

- `core.Notifier.Notify(ctx, repo, build, verdict) error`; `modules/notify.Service{Repos, Notifiers}` dispatches best-effort (logs + swallows errors). `provideNotifyService` (`cmd/server/inject_service.go`) builds the slice: currently `StatusNotifier` then `CommentNotifier`. **Keep `StatusNotifier` first.**
- `core.Verdict{State, Coverage, Change, HasBase, Description}`, `core.StatusState` constants `StatusSuccess`/`StatusFailure`/`StatusError`/`StatusPending`.
- `modules/notify/status.go` `StatusNotifier` has a private `func (n *StatusNotifier) buildURL(repo, build) string` (the only `fmt` user in that file). `modules/notify/comment.go` has a private `func (n *CommentNotifier) target(repo, build) string` and `func statusBadge(state) (emoji, label string)`. Both URL helpers produce the identical `<Server.URL()>/report/<scm>/<fullname>/builds/<number>` string. Task 2 consolidates them.

## File Structure

**Create:**
- `modules/notify/notify_message.go` — `shouldNotify`, `statusLabel`, `summaryLine`, `buildURL` (shared helpers).
- `modules/notify/notify_message_test.go` — helper tests.
- `modules/notify/email.go` — `EmailNotifier` + `buildEmail`.
- `modules/notify/email_test.go` — email tests (injectable sender).
- `modules/notify/slack.go` — `SlackNotifier`.
- `modules/notify/slack_test.go` — slack tests (httptest).
- `web/src/components/SettingsNotifications.vue` — notifications settings card.
- `web/src/components/__tests__/SettingsNotifications.spec.ts` — UI test.

**Modify:**
- `config/config.go` — `SMTP` struct + `Config.SMTP` field + `SMTP.Addr()`.
- `core/repo.go` — three `RepoSetting` fields.
- `modules/notify/status.go` — use shared `buildURL`; drop private method + `fmt` import.
- `modules/notify/comment.go` — use shared `buildURL`; `statusBadge` label from `statusLabel`; drop private `target`.
- `cmd/server/inject_service.go` — append the two notifiers.
- `web/src/types/setting.ts` — three fields.
- `web/src/views/SettingsView.vue` — render the card.
- `web/src/components/__tests__/SettingsChecks.spec.ts`, `SettingsGeneral.spec.ts`, `web/src/stores/__tests__/repository.spec.ts` — fixture updates.

---

## Task 1: config.SMTP + RepoSetting fields

**Files:**
- Modify: `config/config.go`
- Modify: `core/repo.go`

- [ ] **Step 1: Add the SMTP config struct**

In `config/config.go`: add `"net"` to the imports; add an `SMTP SMTP` field to the `Config` struct (after `Server Server`); and add the struct + helper:

```go
// SMTP server setting for outbound notification email.
type SMTP struct {
	Host     string `envconfig:"GATES_SMTP_HOST"`
	Port     string `default:"587" envconfig:"GATES_SMTP_PORT"`
	Username string `envconfig:"GATES_SMTP_USERNAME"`
	Password string `envconfig:"GATES_SMTP_PASSWORD"`
	From     string `envconfig:"GATES_SMTP_FROM"`
}

// Addr is the host:port SMTP dial address.
func (s SMTP) Addr() string {
	return net.JoinHostPort(s.Host, s.Port)
}
```

- [ ] **Step 2: Add the three RepoSetting fields**

In `core/repo.go`, append to `RepoSetting` (after `DisablePRComment`):

```go
	// EmailRecipients receive coverage notification email (empty disables email).
	EmailRecipients []string `json:"emailRecipients"`
	// SlackWebhook is the incoming-webhook URL for Slack notifications (empty disables).
	SlackWebhook string `json:"slackWebhook"`
	// NotifyTrigger controls when email/Slack fire: "off", "failure" (default), "always".
	NotifyTrigger string `json:"notifyTrigger"`
```

- [ ] **Step 3: Build to verify**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./config/... ./core/...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
cd /d/raynorpat/covergates
git add config/config.go core/repo.go
git commit -m "notify: SMTP config and email/slack repo settings"
```

---

## Task 2: Shared helpers + consolidate build-URL

**Files:**
- Create: `modules/notify/notify_message.go`
- Test: `modules/notify/notify_message_test.go`
- Modify: `modules/notify/status.go`
- Modify: `modules/notify/comment.go`

- [ ] **Step 1: Write the failing helper tests**

Create `modules/notify/notify_message_test.go`:

```go
package notify

import (
	"strings"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

func TestShouldNotify(t *testing.T) {
	fail := &core.Verdict{State: core.StatusFailure}
	errd := &core.Verdict{State: core.StatusError}
	ok := &core.Verdict{State: core.StatusSuccess}
	cases := []struct {
		trigger string
		verdict *core.Verdict
		want    bool
	}{
		{"off", fail, false},
		{"always", ok, true},
		{"always", fail, true},
		{"failure", ok, false},
		{"failure", fail, true},
		{"failure", errd, true},
		{"", ok, false},   // empty -> failure default
		{"", fail, true},  // empty -> failure default
	}
	for _, tc := range cases {
		if got := shouldNotify(tc.trigger, tc.verdict); got != tc.want {
			t.Errorf("shouldNotify(%q, %v) = %v, want %v", tc.trigger, tc.verdict.State, got, tc.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	if statusLabel(core.StatusSuccess) != "passed" {
		t.Error("success should be passed")
	}
	if statusLabel(core.StatusFailure) != "failed" {
		t.Error("failure should be failed")
	}
	if statusLabel(core.StatusError) != "errored" {
		t.Error("error should be errored")
	}
}

func TestSummaryLineAndBuildURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	repo := &core.Repo{NameSpace: "o", Name: "r", SCM: core.Github}
	build := &core.Build{Number: 12}
	verdict := &core.Verdict{State: core.StatusFailure, Description: "Coverage decreased 1.0% to 84.0%"}

	line := summaryLine(repo, build, verdict)
	for _, want := range []string{"o/r", "#12", "failed", "Coverage decreased 1.0% to 84.0%"} {
		if !strings.Contains(line, want) {
			t.Errorf("summaryLine %q missing %q", line, want)
		}
	}
	if url := buildURL(cfg, repo, build); url != "http://localhost:8080/report/github/o/r/builds/12" {
		t.Errorf("buildURL = %q", url)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run 'TestShouldNotify|TestStatusLabel|TestSummaryLineAndBuildURL'`
Expected: FAIL — `undefined: shouldNotify` etc.

- [ ] **Step 3: Create the shared helpers**

Create `modules/notify/notify_message.go`:

```go
package notify

import (
	"fmt"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

// Trigger values for RepoSetting.NotifyTrigger.
const (
	TriggerOff     = "off"
	TriggerFailure = "failure"
	TriggerAlways  = "always"
)

// shouldNotify reports whether a verdict warrants an email/Slack notification
// under the given trigger. Empty trigger is treated as "failure" (the default).
func shouldNotify(trigger string, verdict *core.Verdict) bool {
	switch trigger {
	case TriggerOff:
		return false
	case TriggerAlways:
		return true
	default: // TriggerFailure or ""
		return verdict.State == core.StatusFailure || verdict.State == core.StatusError
	}
}

// statusLabel is the plain-text verdict label.
func statusLabel(state core.StatusState) string {
	switch state {
	case core.StatusSuccess:
		return "passed"
	case core.StatusFailure:
		return "failed"
	case core.StatusError:
		return "errored"
	default:
		return "pending"
	}
}

// summaryLine is a one-line human summary used by Slack and the email subject/body.
func summaryLine(repo *core.Repo, build *core.Build, verdict *core.Verdict) string {
	return fmt.Sprintf("%s build #%d %s — %s", repo.FullName(), build.Number, statusLabel(verdict.State), verdict.Description)
}

// buildURL is the build-detail link, shared by all notifiers.
func buildURL(cfg *config.Config, repo *core.Repo, build *core.Build) string {
	return fmt.Sprintf("%s/report/%s/%s/builds/%d", cfg.Server.URL(), repo.SCM, repo.FullName(), build.Number)
}
```

- [ ] **Step 4: Refactor status.go to use the shared buildURL**

In `modules/notify/status.go`:
- Remove the `"fmt"` import (it is used ONLY by the method being removed).
- Replace `Target: n.buildURL(repo, build),` with `Target: buildURL(n.Config, repo, build),`.
- Delete the `func (n *StatusNotifier) buildURL(repo *core.Repo, build *core.Build) string { ... }` method entirely.

- [ ] **Step 5: Refactor comment.go to use the shared buildURL and statusLabel**

In `modules/notify/comment.go`:
- Replace `commentBody(build, verdict, changes, coverageByPath(build), n.target(repo, build))` with `commentBody(build, verdict, changes, coverageByPath(build), buildURL(n.Config, repo, build))`.
- Delete the `func (n *CommentNotifier) target(repo *core.Repo, build *core.Build) string { ... }` method.
- Change `statusBadge` to derive its label from `statusLabel` (keep its emoji):

```go
func statusBadge(state core.StatusState) (emoji, label string) {
	switch state {
	case core.StatusSuccess:
		emoji = "✅"
	case core.StatusFailure:
		emoji = "❌"
	case core.StatusError:
		emoji = "⚠️"
	default:
		emoji = "⏳"
	}
	return emoji, statusLabel(state)
}
```

(`comment.go` still uses `fmt` elsewhere, so keep its `fmt` import.)

- [ ] **Step 6: Run the whole notify package**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/`
Expected: PASS — the new helper tests plus all existing C1/C2 tests (status, comment, verdict, service) still green, confirming the refactor preserved behavior.

- [ ] **Step 7: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/notify_message.go modules/notify/notify_message_test.go modules/notify/status.go modules/notify/comment.go
git commit -m "notify: shared trigger/label/url helpers; dedupe build url"
```

---

## Task 3: EmailNotifier

**Files:**
- Create: `modules/notify/email.go`
- Test: `modules/notify/email_test.go`

- [ ] **Step 1: Write the failing tests**

Create `modules/notify/email_test.go`:

```go
package notify

import (
	"context"
	"net/smtp"
	"strings"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
)

func emailConfig(host, from string) *config.Config {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	cfg.SMTP.Host = host
	cfg.SMTP.Port = "587"
	cfg.SMTP.From = from
	return cfg
}

type capturedMail struct {
	addr string
	from string
	to   []string
	msg  string
	sent bool
}

func emailNotifierWith(cfg *config.Config, repos core.RepoStore, cap *capturedMail) *EmailNotifier {
	return &EmailNotifier{
		Repos:  repos,
		Config: cfg,
		sender: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			cap.sent = true
			cap.addr, cap.from, cap.to, cap.msg = addr, from, to, string(msg)
			return nil
		},
	}
}

func TestEmailNotifierSendsOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{
		NotifyTrigger: "failure", EmailRecipients: []string{"a@x.com", "b@x.com"},
	}, nil)
	build := &core.Build{Number: 12}
	verdict := &core.Verdict{State: core.StatusFailure, Description: "Coverage decreased 1.0% to 84.0%"}

	cap := &capturedMail{}
	n := emailNotifierWith(emailConfig("smtp.x.com", "ci@x.com"), repos, cap)
	if err := n.Notify(context.Background(), repo, build, verdict); err != nil {
		t.Fatalf("Notify error: %v", err)
	}
	if !cap.sent {
		t.Fatal("expected mail to be sent")
	}
	if cap.addr != "smtp.x.com:587" {
		t.Errorf("addr = %q", cap.addr)
	}
	if cap.from != "ci@x.com" || len(cap.to) != 2 {
		t.Errorf("from/to wrong: %q %v", cap.from, cap.to)
	}
	for _, want := range []string{"Subject: [covergates] o/r #12 failed", "Coverage decreased 1.0% to 84.0%", "http://localhost:8080/report/github/o/r/builds/12"} {
		if !strings.Contains(cap.msg, want) {
			t.Errorf("message missing %q\n---\n%s", want, cap.msg)
		}
	}
}

func TestEmailNotifierSkips(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	build := &core.Build{Number: 1}
	success := &core.Verdict{State: core.StatusSuccess, Description: "Coverage: 90.0%"}

	t.Run("trigger off", func(t *testing.T) {
		repos := mock.NewMockRepoStore(ctrl)
		repo := &core.Repo{ID: 1}
		repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "off", EmailRecipients: []string{"a@x.com"}}, nil)
		cap := &capturedMail{}
		n := emailNotifierWith(emailConfig("smtp.x.com", "ci@x.com"), repos, cap)
		if err := n.Notify(context.Background(), repo, build, &core.Verdict{State: core.StatusFailure}); err != nil || cap.sent {
			t.Fatalf("expected no send; err=%v sent=%v", err, cap.sent)
		}
	})
	t.Run("failure trigger but success verdict", func(t *testing.T) {
		repos := mock.NewMockRepoStore(ctrl)
		repo := &core.Repo{ID: 2}
		repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "failure", EmailRecipients: []string{"a@x.com"}}, nil)
		cap := &capturedMail{}
		n := emailNotifierWith(emailConfig("smtp.x.com", "ci@x.com"), repos, cap)
		if err := n.Notify(context.Background(), repo, build, success); err != nil || cap.sent {
			t.Fatalf("expected no send; err=%v sent=%v", err, cap.sent)
		}
	})
	t.Run("no recipients", func(t *testing.T) {
		repos := mock.NewMockRepoStore(ctrl)
		repo := &core.Repo{ID: 3}
		repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "always"}, nil)
		cap := &capturedMail{}
		n := emailNotifierWith(emailConfig("smtp.x.com", "ci@x.com"), repos, cap)
		if err := n.Notify(context.Background(), repo, build, success); err != nil || cap.sent {
			t.Fatalf("expected no send; err=%v sent=%v", err, cap.sent)
		}
	})
	t.Run("smtp not configured", func(t *testing.T) {
		repos := mock.NewMockRepoStore(ctrl)
		repo := &core.Repo{ID: 4}
		repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "always", EmailRecipients: []string{"a@x.com"}}, nil)
		cap := &capturedMail{}
		n := emailNotifierWith(emailConfig("", ""), repos, cap) // no host/from
		if err := n.Notify(context.Background(), repo, build, success); err != nil || cap.sent {
			t.Fatalf("expected no send; err=%v sent=%v", err, cap.sent)
		}
	})
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestEmailNotifier`
Expected: FAIL — `undefined: EmailNotifier`.

- [ ] **Step 3: Implement**

Create `modules/notify/email.go`:

```go
package notify

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

type smtpSender func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// EmailNotifier sends a coverage summary email to the repo's configured
// recipients via the server SMTP relay, gated by the repo's notify trigger.
type EmailNotifier struct {
	Repos  core.RepoStore
	Config *config.Config
	sender smtpSender // nil -> smtp.SendMail
}

// Notify sends the email when the trigger passes, recipients exist, and SMTP is configured.
func (n *EmailNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	setting, err := n.Repos.Setting(repo)
	if err != nil || setting == nil {
		return nil
	}
	if !shouldNotify(setting.NotifyTrigger, verdict) || len(setting.EmailRecipients) == 0 {
		return nil
	}
	s := n.Config.SMTP
	if s.Host == "" || s.From == "" {
		return nil
	}
	msg := buildEmail(s.From, setting.EmailRecipients, repo, build, verdict, buildURL(n.Config, repo, build))
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	send := n.sender
	if send == nil {
		send = smtp.SendMail
	}
	return send(s.Addr(), auth, s.From, setting.EmailRecipients, msg)
}

func buildEmail(from string, to []string, repo *core.Repo, build *core.Build, verdict *core.Verdict, url string) []byte {
	subject := fmt.Sprintf("[covergates] %s #%d %s", repo.FullName(), build.Number, statusLabel(verdict.State))
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "%s\r\n\r\n%s\r\n", verdict.Description, url)
	return []byte(b.String())
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestEmailNotifier`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/email.go modules/notify/email_test.go
git commit -m "notify: email notifier"
```

---

## Task 4: SlackNotifier

**Files:**
- Create: `modules/notify/slack.go`
- Test: `modules/notify/slack_test.go`

- [ ] **Step 1: Write the failing tests**

Create `modules/notify/slack_test.go`:

```go
package notify

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
)

func slackConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	return cfg
}

func TestSlackNotifierPosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		gotBody = string(b)
		gotType = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "always", SlackWebhook: srv.URL}, nil)
	build := &core.Build{Number: 12}
	verdict := &core.Verdict{State: core.StatusSuccess, Description: "Coverage: 90.0%"}

	n := &SlackNotifier{Repos: repos, Config: slackConfig(), Client: srv.Client()}
	if err := n.Notify(context.Background(), repo, build, verdict); err != nil {
		t.Fatalf("Notify error: %v", err)
	}
	if gotType != "application/json" {
		t.Errorf("content-type = %q", gotType)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatalf("bad json: %v (%s)", err, gotBody)
	}
	for _, want := range []string{"o/r", "#12", "Coverage: 90.0%", "http://localhost:8080/report/github/o/r/builds/12"} {
		if !strings.Contains(payload["text"], want) {
			t.Errorf("text %q missing %q", payload["text"], want)
		}
	}
}

func TestSlackNotifierSkipsWhenNoWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "always"}, nil) // no webhook
	n := &SlackNotifier{Repos: repos, Config: slackConfig()}
	if err := n.Notify(context.Background(), repo, &core.Build{Number: 1}, &core.Verdict{State: core.StatusSuccess}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestSlackNotifierErrorsOnNon2xx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{NotifyTrigger: "always", SlackWebhook: srv.URL}, nil)
	n := &SlackNotifier{Repos: repos, Config: slackConfig(), Client: srv.Client()}
	if err := n.Notify(context.Background(), repo, &core.Build{Number: 1}, &core.Verdict{State: core.StatusFailure}); err == nil {
		t.Fatal("expected error on non-2xx response")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestSlackNotifier`
Expected: FAIL — `undefined: SlackNotifier`.

- [ ] **Step 3: Implement**

Create `modules/notify/slack.go`:

```go
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

// SlackNotifier posts a coverage summary to a repo's Slack incoming webhook,
// gated by the repo's notify trigger.
type SlackNotifier struct {
	Repos  core.RepoStore
	Config *config.Config
	Client *http.Client // nil -> http.DefaultClient
}

// Notify posts to the webhook when the trigger passes and a webhook is configured.
func (n *SlackNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	setting, err := n.Repos.Setting(repo)
	if err != nil || setting == nil {
		return nil
	}
	if !shouldNotify(setting.NotifyTrigger, verdict) || setting.SlackWebhook == "" {
		return nil
	}
	text := fmt.Sprintf("%s\n%s", summaryLine(repo, build, verdict), buildURL(n.Config, repo, build))
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.SlackWebhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/`
Expected: PASS (all notify tests).

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/slack.go modules/notify/slack_test.go
git commit -m "notify: slack webhook notifier"
```

---

## Task 5: Wire Email + Slack notifiers

**Files:**
- Modify: `cmd/server/inject_service.go`

- [ ] **Step 1: Append both notifiers**

In `cmd/server/inject_service.go`, in `provideNotifyService`, add the two notifiers to the `Notifiers` slice AFTER the existing `StatusNotifier` and `CommentNotifier`:

```go
			&notifymod.CommentNotifier{
				SCM:    scmService,
				Repos:  repoStore,
				Config: config,
			},
			&notifymod.EmailNotifier{
				Repos:  repoStore,
				Config: config,
			},
			&notifymod.SlackNotifier{
				Repos:  repoStore,
				Config: config,
			},
```

(`EmailNotifier.sender` and `SlackNotifier.Client` stay nil → stdlib defaults. The provider already has `config` and `repoStore`; no signature or `wire_gen.go` change.)

- [ ] **Step 2: Build**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./core/... ./modules/... ./routers/... ./cmd/... ./mock/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
cd /d/raynorpat/covergates
git add cmd/server/inject_service.go
git commit -m "notify: register email and slack notifiers"
```

---

## Task 6: Frontend Notifications settings card

**Files:**
- Modify: `web/src/types/setting.ts`
- Create: `web/src/components/SettingsNotifications.vue`
- Modify: `web/src/views/SettingsView.vue`
- Test: `web/src/components/__tests__/SettingsNotifications.spec.ts`
- Modify: `web/src/components/__tests__/SettingsChecks.spec.ts`, `web/src/components/__tests__/SettingsGeneral.spec.ts`, `web/src/stores/__tests__/repository.spec.ts`

- [ ] **Step 1: Add the type fields**

Edit `web/src/types/setting.ts` — add after `disablePRComment`:

```ts
  disablePRComment: boolean
  emailRecipients: string[]
  slackWebhook: string
  notifyTrigger: string
```

- [ ] **Step 2: Write the failing component test**

Create `web/src/components/__tests__/SettingsNotifications.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsNotifications from '@/components/SettingsNotifications.vue'
import type { RepoSetting } from '@/types/setting'

const vuetify = createVuetify({ components, directives })

const setting: RepoSetting = {
  filters: [], mergePR: false, updateAction: '', protected: false,
  coverageMinimum: 0, coverageDecreaseThreshold: 0, disablePRComment: false,
  emailRecipients: ['a@x.com'], slackWebhook: '', notifyTrigger: 'failure'
}

describe('SettingsNotifications', () => {
  it('emits save with edited recipients and webhook', async () => {
    const w = mount(SettingsNotifications, {
      props: { modelValue: setting },
      global: { plugins: [vuetify] }
    })
    const textarea = w.find('textarea')
    await textarea.setValue('a@x.com\nb@x.com\n')
    const text = w.find('input[type="text"]')
    await text.setValue('https://hooks.slack.com/abc')
    await w.find('button').trigger('click')
    const events = w.emitted('save') as RepoSetting[][]
    expect(events).toBeTruthy()
    expect(events[0][0].emailRecipients).toEqual(['a@x.com', 'b@x.com'])
    expect(events[0][0].slackWebhook).toBe('https://hooks.slack.com/abc')
    // unrelated fields preserved
    expect(events[0][0].notifyTrigger).toBe('failure')
  })
})
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/components/__tests__/SettingsNotifications.spec.ts`
Expected: FAIL — cannot resolve `@/components/SettingsNotifications.vue`.

- [ ] **Step 4: Implement the component**

Create `web/src/components/SettingsNotifications.vue`:

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const trigger = ref(props.modelValue.notifyTrigger || 'failure')
const recipientsText = ref((props.modelValue.emailRecipients ?? []).join('\n'))
const slackWebhook = ref(props.modelValue.slackWebhook ?? '')

const triggerItems = [
  { title: 'Off', value: 'off' },
  { title: 'On failure', value: 'failure' },
  { title: 'Always', value: 'always' }
]

watch(() => props.modelValue, (v) => {
  trigger.value = v.notifyTrigger || 'failure'
  recipientsText.value = (v.emailRecipients ?? []).join('\n')
  slackWebhook.value = v.slackWebhook ?? ''
})

function save() {
  emit('save', {
    ...props.modelValue,
    notifyTrigger: trigger.value,
    emailRecipients: recipientsText.value.split('\n').map((s) => s.trim()).filter(Boolean),
    slackWebhook: slackWebhook.value.trim()
  })
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Notifications</div>
    <p class="text-body-2 text-medium-emphasis mb-3">
      Email and Slack delivery for finalized builds.
    </p>
    <v-select v-model="trigger" :items="triggerItems" label="When to notify" density="compact" />
    <v-textarea v-model="recipientsText" label="Email recipients (one per line)" rows="3" auto-grow />
    <v-text-field v-model="slackWebhook" type="text" label="Slack webhook URL" density="compact" />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/components/__tests__/SettingsNotifications.spec.ts`
Expected: PASS.

If `w.find('input[type="text"]')` is ambiguous (the `v-select` may also render a text input), target the Slack field more specifically — e.g. `w.findAll('input[type="text"]')` and pick the last, or add a stable selector. The test must edit the recipients textarea and the Slack webhook field, then assert the emitted payload.

- [ ] **Step 6: Render the card in the settings view**

In `web/src/views/SettingsView.vue`: add the import alongside the others:

```ts
import SettingsNotifications from '@/components/SettingsNotifications.vue'
```

and render it right after `<SettingsChecks ... />`, reusing the same handler/flag:

```html
      <SettingsChecks :model-value="store.setting" :busy="saving" @save="save" />
      <SettingsNotifications :model-value="store.setting" :busy="saving" @save="save" />
```

- [ ] **Step 7: Update the other RepoSetting fixtures**

The `RepoSetting` interface now requires `emailRecipients`, `slackWebhook`, `notifyTrigger`. Add them to the three existing fixtures so the type-check build passes:

- `web/src/components/__tests__/SettingsChecks.spec.ts`: in its `setting` literal add `, emailRecipients: [], slackWebhook: '', notifyTrigger: 'failure'`.
- `web/src/components/__tests__/SettingsGeneral.spec.ts`: in its `modelValue` literal add `, emailRecipients: [], slackWebhook: '', notifyTrigger: 'failure'`.
- `web/src/stores/__tests__/repository.spec.ts`: in its `const s` literal add `, emailRecipients: [], slackWebhook: '', notifyTrigger: 'failure'`.

- [ ] **Step 8: Run the full suite, lint, and type-check build**

Run:
```
cd /d/raynorpat/covergates/web && npx vitest run && npm run lint && npm run build
```
Expected: all suites pass; lint clean; `npm run build` succeeds (only the pre-existing chunk-size warning).

- [ ] **Step 9: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/types/setting.ts web/src/components/SettingsNotifications.vue web/src/views/SettingsView.vue web/src/components/__tests__/SettingsNotifications.spec.ts web/src/components/__tests__/SettingsChecks.spec.ts web/src/components/__tests__/SettingsGeneral.spec.ts web/src/stores/__tests__/repository.spec.ts
git commit -m "web: notifications settings card (email, slack, trigger)"
```

---

## Task 7: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Backend vet + build + targeted tests**

Run:
```
cd /d/raynorpat/covergates && CGO_ENABLED=0 go vet ./core/... ./config/... ./modules/notify/... ./routers/api/build/... ./cmd/... 2>&1 | tail -20
CGO_ENABLED=0 go build ./core/... ./modules/... ./routers/... ./cmd/... ./mock/...
CGO_ENABLED=0 go test ./modules/notify/ ./modules/scm/ ./routers/api/build/ ./modules/build/
```
Expected: vet clean; build clean; all four packages `ok`. (`models` is excluded — requires CGO/sqlite.)

- [ ] **Step 2: Frontend lint, test, build**

Run:
```
cd /d/raynorpat/covergates/web && npm run lint && npx vitest run && npm run build
```
Expected: lint clean; all Vitest suites pass; production build succeeds.

- [ ] **Step 3: Final commit (only if a verification fix was needed)**

```bash
cd /d/raynorpat/covergates
git add -A
git commit -m "notify: C3 verification fixes"
```
(Skip if nothing changed.)

---

## Done criteria

- A finalized build sends an email (to per-repo recipients via server SMTP) and/or a Slack webhook message when the per-repo trigger (`off`/`failure`/`always`, default failure) permits.
- A channel with no target (no recipients / no webhook / SMTP unconfigured) or `trigger=off` is a silent no-op; delivery failures never affect the upload (best-effort dispatch).
- The Settings page exposes a Notifications card (trigger select, recipients, Slack webhook).
- The build-URL string lives in one shared helper used by all three notifiers; all targeted Go packages and the full frontend suite pass; `go vet` clean; the frontend type-check build succeeds.
