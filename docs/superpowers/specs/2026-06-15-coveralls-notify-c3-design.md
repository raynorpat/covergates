# Coveralls Notifications & Checks — C3 (Email + Slack Delivery) Design

**Date:** 2026-06-15
**Sub-project:** C (notifications & checks), chunk **C3** of 3 (final).
**Branch:** `feat/coveralls-notify-c3`

## Goal

After a build finalizes, optionally deliver a coverage summary over email (server-wide SMTP,
per-repo recipients) and/or Slack (per-repo incoming webhook), gated by a per-repo trigger
(off / on-failure / always). Completes the coveralls-style notification surface.

## Context

- C1 established the dispatch: `modules/notify.Service` (best-effort) computes a
  `core.Verdict{State, Coverage, Change, HasBase, Description}` and fans out to
  `[]core.Notifier` after `Finalize`. C1 added `StatusNotifier`; C2 added `CommentNotifier`.
  C3 adds `EmailNotifier` and `SlackNotifier` to the same slice — no handler/wiring changes
  beyond appending them in `provideNotifyService` (`cmd/server/inject_service.go`).
- `core.Notifier.Notify(ctx, repo, build, verdict)` is the interface; the dispatcher does
  not pass `RepoSetting`, so each notifier reads its own config via `RepoStore.Setting(repo)`
  (same approach as `CommentNotifier`).
- Notifications are best-effort: the dispatcher logs and swallows any notifier error
  (`modules/notify/service.go`); a notifier failure never affects the upload/webhook response
  or other notifiers. `StatusNotifier` is FIRST in the slice and must stay first.
- `config.Config` is populated by `kelseyhightower/envconfig` from env vars; each section is a
  struct with `envconfig`/`default` tags (see `config/config.go`). `cfg.Server.URL()` yields
  the server base URL; the build-detail link format used by C1/C2 is
  `<Server.URL()>/report/<scm>/<fullname>/builds/<number>`.
- `core.RepoSetting` is a JSON blob (`models.RepoSetting.Config`); new fields need no migration.
- Go 1.14 stdlib only: `net/smtp` for email, `net/http` for the Slack webhook. No new deps.
- The frontend settings page composes cards in `web/src/views/SettingsView.vue`, each bound to
  the shared `store.setting` + a single `save` handler (see `SettingsChecks.vue` / B3a pattern).

## Decisions (from brainstorming)

1. **Trigger:** per-repo `notifyTrigger` ∈ {`off`, `failure`, `always`}; empty ⇒ `failure`
   (default). One trigger shared by both channels. `failure` fires when
   `verdict.State` is `failure` or `error`.
2. **Email config:** server-wide SMTP (host/port/user/pass/from via env), per-repo recipient
   list. Email is sent only when SMTP is configured AND recipients are non-empty AND the
   trigger passes. Applies to ALL finalized builds (not just PRs).
3. **Slack config:** per-repo incoming-webhook URL. Sent only when the URL is set AND the
   trigger passes.
4. **No new master toggle:** `notifyTrigger=off` (or empty target) disables a channel; this
   is the off-switch.
5. **Testability seams:** `EmailNotifier` sends through an injectable function field
   defaulting to `smtp.SendMail`; `SlackNotifier` posts through an injected `*http.Client`
   (default `http.DefaultClient`), tested against `httptest.Server`.

## Architecture

```
notify.Service.Notify(ctx, repo, build)            # after Finalize
  verdict = Evaluate(build, setting)
  for n in Notifiers:  # StatusNotifier, CommentNotifier, EmailNotifier, SlackNotifier
    n.Notify(ctx, repo, build, verdict)             # errors logged + swallowed

EmailNotifier.Notify:
  setting = Repos.Setting(repo)
  if !shouldNotify(setting.NotifyTrigger, verdict): return nil
  if len(setting.EmailRecipients) == 0:             return nil
  if cfg.SMTP.Host == "" || cfg.SMTP.From == "":    return nil
  msg = buildEmail(cfg.SMTP.From, recipients, repo, build, verdict, target)
  return n.sender(addr, auth, cfg.SMTP.From, recipients, msg)

SlackNotifier.Notify:
  setting = Repos.Setting(repo)
  if !shouldNotify(setting.NotifyTrigger, verdict): return nil
  if setting.SlackWebhook == "":                    return nil
  body = {"text": summaryLine(repo, build, verdict, target)}
  POST setting.SlackWebhook  (non-2xx -> error)
```

### Component 1 — config.SMTP

`config/config.go` — add the struct and a field on `Config`:

```go
type Config struct {
	Server   Server
	SMTP     SMTP
	Gitea    Gitea
	// ... existing fields ...
}

// SMTP server setting for outbound notification email.
type SMTP struct {
	Host     string `envconfig:"GATES_SMTP_HOST"`
	Port     string `default:"587" envconfig:"GATES_SMTP_PORT"`
	Username string `envconfig:"GATES_SMTP_USERNAME"`
	Password string `envconfig:"GATES_SMTP_PASSWORD"`
	From     string `envconfig:"GATES_SMTP_FROM"`
}

// Addr is the host:port SMTP dial address.
func (s SMTP) Addr() string { return net.JoinHostPort(s.Host, s.Port) }
```

### Component 2 — RepoSetting fields

`core/repo.go` — append to `RepoSetting`:

```go
	// EmailRecipients receive coverage notification email (empty disables email).
	EmailRecipients []string `json:"emailRecipients"`
	// SlackWebhook is the incoming-webhook URL for Slack notifications (empty disables).
	SlackWebhook string `json:"slackWebhook"`
	// NotifyTrigger controls when email/Slack fire: "off", "failure" (default), "always".
	NotifyTrigger string `json:"notifyTrigger"`
```

### Component 3 — shared helpers (`modules/notify/notify_message.go`)

```go
// Trigger values for RepoSetting.NotifyTrigger.
const (
	TriggerOff     = "off"
	TriggerFailure = "failure"
	TriggerAlways  = "always"
)

// shouldNotify reports whether a verdict warrants a notification under the trigger.
// Empty trigger is treated as "failure" (the default).
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

// summaryLine is a one-line human summary used by Slack and the email subject/body.
func summaryLine(repo *core.Repo, build *core.Build, verdict *core.Verdict) string {
	return fmt.Sprintf("%s build #%d %s — %s", repo.FullName(), build.Number, statusLabel(verdict.State), verdict.Description)
}

// statusLabel is the plain-text verdict label ("passed"/"failed"/"errored"/"pending").
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

// buildURL is the build-detail link (shared format with status/comment notifiers).
func buildURL(cfg *config.Config, repo *core.Repo, build *core.Build) string {
	return fmt.Sprintf("%s/report/%s/%s/builds/%d", cfg.Server.URL(), repo.SCM, repo.FullName(), build.Number)
}
```

`statusLabel` is the single source of the label vocabulary — refactor the existing
`statusBadge` in `comment.go` to derive its label from `statusLabel` (its emoji stays in
`comment.go`).

This also resolves the C2 review note about the duplicated build-URL helper: `status.go` and
`comment.go` should both call this shared `buildURL` instead of their private copies.

### Component 4 — EmailNotifier (`modules/notify/email.go`)

```go
type smtpSender func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

type EmailNotifier struct {
	Repos  core.RepoStore
	Config *config.Config
	sender smtpSender // nil -> smtp.SendMail
}

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
```

`buildEmail` (pure, tested) renders an RFC 822 message:

```
From: <from>
To: <r1, r2, ...>
Subject: [covergates] <ns>/<name> #<number> <label>
MIME-Version: 1.0
Content-Type: text/plain; charset=UTF-8

<verdict.Description>

<buildURL>
```

### Component 5 — SlackNotifier (`modules/notify/slack.go`)

```go
type SlackNotifier struct {
	Repos  core.RepoStore
	Config *config.Config
	Client *http.Client // nil -> http.DefaultClient
}

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

### Component 6 — wiring

`cmd/server/inject_service.go` — append to the `Notifiers` slice in `provideNotifyService`
(after `StatusNotifier` and `CommentNotifier`):

```go
&notifymod.EmailNotifier{Repos: repoStore, Config: config},
&notifymod.SlackNotifier{Repos: repoStore, Config: config},
```

(`sender`/`Client` left nil → stdlib defaults. The provider already has `config` and
`repoStore`; no signature or `wire_gen.go` change.)

### Component 7 — frontend Notifications card

- `web/src/types/setting.ts` — add `emailRecipients: string[]`, `slackWebhook: string`,
  `notifyTrigger: string`.
- `web/src/components/SettingsNotifications.vue` (new) — mirrors `SettingsChecks.vue`
  (props `{modelValue, busy}`, emits `save`, refs synced via `watch`, save spreads
  `...modelValue`). Controls:
  - trigger: `v-select` with items Off (`off`) / On failure (`failure`) / Always (`always`).
  - email recipients: `v-textarea` (one per line; split/trim/filter on save, like the filters
    field in `SettingsGeneral.vue`).
  - Slack webhook: `v-text-field`.
- `web/src/views/SettingsView.vue` — render `<SettingsNotifications>` after
  `<SettingsChecks>`, reusing the same `save` handler and `saving` flag.
- Update the 3 `RepoSetting` test fixtures (`SettingsChecks.spec.ts`, `SettingsGeneral.spec.ts`,
  `repository.spec.ts`) with the new fields.

## Data flow

1. Build finalizes (Coverage/Change/Status/Number set; PR or branch).
2. Handler calls `notifyService.Notify`; dispatcher computes verdict, runs every notifier.
3. `EmailNotifier`/`SlackNotifier` each read the repo setting, apply the trigger gate, and
   deliver to their configured target.
4. Any error is logged best-effort; the HTTP response and other notifiers are unaffected.

## Error handling

- Trigger `off`, empty recipients/webhook, or unconfigured SMTP → return nil (no-op).
- Setting read error → return nil (no notification rather than a spurious send).
- SMTP send error / Slack non-2xx or transport error → returned; dispatcher logs it.
- `json.Marshal` of the Slack payload cannot fail for a `map[string]string`; the error is
  ignored intentionally.

## Testing

- `modules/notify/notify_message_test.go` — `shouldNotify` table (off/failure/always × success/
  failure/error/empty-trigger); `summaryLine` content; `buildURL` format.
- `modules/notify/email_test.go` — inject a fake `sender` capturing args:
  - trigger gate (off → no send; failure+success verdict → no send; failure+failure → send;
    always+success → send).
  - no recipients → no send; SMTP unconfigured (no Host/From) → no send.
  - on send: correct `addr`, `from`, recipient list, and the message contains the Subject with
    label, the verdict description, and the build URL; `PlainAuth` used only when Username set.
- `modules/notify/slack_test.go` — `httptest.Server` capturing the request:
  - gate + empty-webhook no-op; on send: POST, `application/json`, body `{"text":...}` containing
    the summary and URL; non-2xx response → error returned.
- `web` — a `SettingsNotifications.spec.ts` that edits trigger/recipients/webhook and asserts the
  emitted payload; fixtures updated so `npm run build` type-check passes.
- `config` — `SMTP.Addr()` join test (host+port) if a `config` test file exists; otherwise covered
  by build.

## Out of scope

- HTML email, templated/branded messages, attachments.
- Slack Block Kit formatting (plain `text` only), Slack OAuth apps (incoming webhook only).
- Per-recipient or per-channel distinct triggers (one shared trigger).
- Retry/queue for failed deliveries (best-effort, single attempt).
- Custom TLS / skip-verify for SMTP (relies on `smtp.SendMail`'s STARTTLS-if-available behavior;
  a self-signed relay needing skipped verification is a follow-up).
