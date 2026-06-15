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
