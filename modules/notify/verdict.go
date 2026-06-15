package notify

import (
	"fmt"

	"github.com/covergates/covergates/core"
)

// Evaluate derives the commit-status verdict for a finalized build under the
// repository's coverage policy. Coverage percentages are whole numbers; the
// build's Coverage/CoverageChange are ratios in [0,1].
func Evaluate(build *core.Build, setting *core.RepoSetting) *core.Verdict {
	v := &core.Verdict{
		Coverage: build.Coverage,
		Change:   build.CoverageChange,
		HasBase:  build.BaseBuildID > 0,
	}
	if build.Status == core.BuildErrored {
		v.State = core.StatusError
		v.Description = "Coverage build failed"
		return v
	}

	covPct := build.Coverage * 100
	v.State = core.StatusSuccess

	if setting != nil && setting.CoverageMinimum > 0 && covPct < setting.CoverageMinimum {
		v.State = core.StatusFailure
		v.Description = fmt.Sprintf("Coverage %.1f%% is below the minimum %.1f%%", covPct, setting.CoverageMinimum)
		return v
	}

	if v.HasBase {
		decreasePct := -build.CoverageChange * 100 // positive when coverage dropped
		threshold := 0.0
		if setting != nil {
			threshold = setting.CoverageDecreaseThreshold
		}
		if decreasePct > threshold {
			v.State = core.StatusFailure
			v.Description = fmt.Sprintf("Coverage decreased %.1f%% to %.1f%%", decreasePct, covPct)
			return v
		}
	}

	v.Description = describePass(covPct, build.CoverageChange, v.HasBase)
	return v
}

func describePass(covPct, change float64, hasBase bool) string {
	if hasBase {
		return fmt.Sprintf("Coverage: %.1f%% (%+.1f%%)", covPct, change*100)
	}
	return fmt.Sprintf("Coverage: %.1f%%", covPct)
}
