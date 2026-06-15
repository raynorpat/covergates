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
