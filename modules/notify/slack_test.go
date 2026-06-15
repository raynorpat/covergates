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
