package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

// CommentNotifier posts a rolling coverage comment to a build's pull request,
// using the repository creator's SCM token. go-scm has no comment edit, so it
// deletes the previously tracked comment before posting a fresh one.
type CommentNotifier struct {
	SCM    core.SCMService
	Repos  core.RepoStore
	Config *config.Config
}

// Notify posts/refreshes the PR coverage comment. No-op for non-PR builds or
// when DisablePRComment is set.
func (n *CommentNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	if build.PullRequest <= 0 {
		return nil
	}
	setting, err := n.Repos.Setting(repo)
	if err == nil && setting != nil && setting.DisablePRComment {
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
	prs := client.PullRequests()
	changes, err := prs.ListChanges(ctx, user, repo.FullName(), build.PullRequest)
	if err != nil {
		return err
	}
	body := commentBody(build, verdict, changes, coverageByPath(build), buildURL(n.Config, repo, build))

	if prev, err := n.Repos.FindPullRequestComment(repo.ID, build.PullRequest); err == nil && prev > 0 {
		// Best-effort delete of the previous comment; it may already be gone.
		_ = prs.RemoveComment(ctx, user, repo.FullName(), build.PullRequest, prev)
	}
	id, err := prs.CreateComment(ctx, user, repo.FullName(), build.PullRequest, body)
	if err != nil {
		return err
	}
	return n.Repos.UpdatePullRequestComment(repo.ID, build.PullRequest, id)
}

// coverageByPath maps each build source file to its coverage ratio in [0,1],
// or -1 when the file has no relevant (executable) lines.
func coverageByPath(build *core.Build) map[string]float64 {
	m := make(map[string]float64, len(build.SourceFiles))
	for _, f := range build.SourceFiles {
		relevant, covered := 0, 0
		for _, hit := range f.Coverage {
			if hit == nil {
				continue
			}
			relevant++
			if *hit > 0 {
				covered++
			}
		}
		if relevant == 0 {
			m[f.Name] = -1
			continue
		}
		m[f.Name] = float64(covered) / float64(relevant)
	}
	return m
}

// commentBody renders the PR coverage comment markdown.
func commentBody(build *core.Build, verdict *core.Verdict,
	changes []*core.FileChange, cov map[string]float64, target string) string {
	var b strings.Builder
	emoji, label := statusBadge(verdict.State)
	fmt.Fprintf(&b, "## Coverage %s %s\n", emoji, label)
	fmt.Fprintf(&b, "%s\n\n", verdict.Description)
	fmt.Fprintf(&b, "[Build #%d](%s)", build.Number, target)
	if build.BaseBuildNumber > 0 {
		fmt.Fprintf(&b, " · base #%d", build.BaseBuildNumber)
	}
	b.WriteString("\n")

	if len(changes) > 0 {
		b.WriteString("\n### Changed files\n")
		b.WriteString("| File | Coverage |\n| --- | --- |\n")
		for _, ch := range changes {
			fmt.Fprintf(&b, "| %s | %s |\n", strings.ReplaceAll(ch.Path, "|", `\|`), coverageCell(ch, cov))
		}
	}
	return b.String()
}

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

func coverageCell(ch *core.FileChange, cov map[string]float64) string {
	if ch.Deleted {
		return "deleted"
	}
	if ratio, ok := cov[ch.Path]; ok && ratio >= 0 {
		return fmt.Sprintf("%.1f%%", ratio*100)
	}
	return "—"
}
