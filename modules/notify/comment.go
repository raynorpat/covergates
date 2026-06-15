package notify

import (
	"fmt"
	"strings"

	"github.com/covergates/covergates/core"
)

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
		return "✅", "passed"
	case core.StatusFailure:
		return "❌", "failed"
	case core.StatusError:
		return "⚠️", "errored"
	default:
		return "⏳", "pending"
	}
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
