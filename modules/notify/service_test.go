package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
)

type recordingNotifier struct {
	called  bool
	verdict *core.Verdict
	err     error
}

func (n *recordingNotifier) Notify(_ context.Context, _ *core.Repo, _ *core.Build, v *core.Verdict) error {
	n.called = true
	n.verdict = v
	return n.err
}

func TestServiceDispatchesAndSwallowsErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1}
	build := &core.Build{Status: core.BuildDone, Coverage: 0.5, CoverageChange: -0.1, BaseBuildID: 3}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{}, nil)

	failing := &recordingNotifier{err: errors.New("boom")}
	ok := &recordingNotifier{}
	svc := &Service{Repos: repos, Notifiers: []core.Notifier{failing, ok}}

	if err := svc.Notify(context.Background(), repo, build); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if !failing.called || !ok.called {
		t.Fatal("expected all notifiers to be called")
	}
	if ok.verdict == nil || ok.verdict.State != core.StatusFailure {
		t.Fatalf("expected failure verdict passed to notifier, got %+v", ok.verdict)
	}
}

func TestServiceTreatsSettingErrorAsEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1}
	build := &core.Build{Status: core.BuildDone, Coverage: 0.9}
	repos.EXPECT().Setting(repo).Return(nil, errors.New("db down"))

	n := &recordingNotifier{}
	svc := &Service{Repos: repos, Notifiers: []core.Notifier{n}}
	if err := svc.Notify(context.Background(), repo, build); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if n.verdict == nil || n.verdict.State != core.StatusSuccess {
		t.Fatalf("expected success verdict, got %+v", n.verdict)
	}
}
