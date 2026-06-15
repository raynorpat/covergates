package notify

import (
	"context"

	"github.com/covergates/covergates/core"
	log "github.com/sirupsen/logrus"
)

// Service computes a build's verdict and dispatches it to each notifier.
// Notification is best-effort: a notifier error is logged, never returned.
type Service struct {
	Repos     core.RepoStore
	Notifiers []core.Notifier
}

// Notify evaluates the build under the repo's policy and runs every notifier.
func (s *Service) Notify(ctx context.Context, repo *core.Repo, build *core.Build) error {
	setting, err := s.Repos.Setting(repo)
	if err != nil || setting == nil {
		setting = &core.RepoSetting{}
	}
	verdict := Evaluate(build, setting)
	for _, n := range s.Notifiers {
		if err := n.Notify(ctx, repo, build, verdict); err != nil {
			log.Errorf("notify: notifier failed: %v", err)
		}
	}
	return nil
}
