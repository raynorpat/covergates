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
