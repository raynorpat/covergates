package notify

import (
	"strings"
	"testing"

	"github.com/covergates/covergates/core"
)

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name      string
		build     *core.Build
		setting   *core.RepoSetting
		wantState core.StatusState
		wantDesc  string // substring
	}{
		{
			name:      "errored build",
			build:     &core.Build{Status: core.BuildErrored, Coverage: 0.5},
			setting:   &core.RepoSetting{},
			wantState: core.StatusError,
			wantDesc:  "build failed",
		},
		{
			name:      "no base no minimum passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.853},
			setting:   &core.RepoSetting{},
			wantState: core.StatusSuccess,
			wantDesc:  "85.3%",
		},
		{
			name:      "base no decrease passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.90, CoverageChange: 0.05, BaseBuildID: 7},
			setting:   &core.RepoSetting{},
			wantState: core.StatusSuccess,
			wantDesc:  "+5.0%",
		},
		{
			name:      "any decrease fails with default threshold",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.84, CoverageChange: -0.012, BaseBuildID: 7},
			setting:   &core.RepoSetting{},
			wantState: core.StatusFailure,
			wantDesc:  "decreased",
		},
		{
			name:      "decrease within threshold passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.84, CoverageChange: -0.01, BaseBuildID: 7},
			setting:   &core.RepoSetting{CoverageDecreaseThreshold: 2},
			wantState: core.StatusSuccess,
			wantDesc:  "-1.0%",
		},
		{
			name:      "below minimum fails",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.40},
			setting:   &core.RepoSetting{CoverageMinimum: 80},
			wantState: core.StatusFailure,
			wantDesc:  "below the minimum",
		},
		{
			name:      "minimum takes precedence over decrease",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.40, CoverageChange: -0.10, BaseBuildID: 7},
			setting:   &core.RepoSetting{CoverageMinimum: 80, CoverageDecreaseThreshold: 5},
			wantState: core.StatusFailure,
			wantDesc:  "below the minimum",
		},
		{
			name:      "exactly at minimum passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.80},
			setting:   &core.RepoSetting{CoverageMinimum: 80},
			wantState: core.StatusSuccess,
			wantDesc:  "80.0%",
		},
		{
			name:      "nil setting no base passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.70},
			setting:   nil,
			wantState: core.StatusSuccess,
			wantDesc:  "70.0%",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(tc.build, tc.setting)
			if v.State != tc.wantState {
				t.Fatalf("state = %q, want %q", v.State, tc.wantState)
			}
			if !strings.Contains(v.Description, tc.wantDesc) {
				t.Fatalf("description %q does not contain %q", v.Description, tc.wantDesc)
			}
		})
	}
}
