package notify

import (
	"context"
	"fmt"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

// StatusNotifier posts a commit status reflecting the coverage verdict, using
// the repository creator's SCM token.
type StatusNotifier struct {
	SCM    core.SCMService
	Repos  core.RepoStore
	Config *config.Config
}

// Notify posts the verdict as a commit status on the build's commit.
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
		Target: n.buildURL(repo, build),
	}
	return client.Repositories().CreateStatus(ctx, user, repo.FullName(), build.Commit, status)
}

func (n *StatusNotifier) buildURL(repo *core.Repo, build *core.Build) string {
	return fmt.Sprintf("%s/report/%s/%s/builds/%d", n.Config.Server.URL(), repo.SCM, repo.FullName(), build.Number)
}
