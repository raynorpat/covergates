package notify

import (
	"strings"
	"testing"

	"github.com/covergates/covergates/core"
)

func ptr(i int) *int { return &i }

func TestCoverageByPath(t *testing.T) {
	build := &core.Build{SourceFiles: []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{ptr(1), ptr(0)}}, // 1/2 = 0.5
		{Name: "b.go", Coverage: []*int{ptr(2), ptr(3)}}, // both >0 -> 2/2 = 1.0
		{Name: "c.go", Coverage: []*int{nil, nil}},       // no relevant lines -> -1
	}}
	cov := coverageByPath(build)
	if cov["a.go"] != 0.5 {
		t.Errorf("a.go = %v, want 0.5", cov["a.go"])
	}
	if cov["b.go"] != 1.0 {
		t.Errorf("b.go = %v, want 1.0", cov["b.go"])
	}
	if cov["c.go"] != -1 {
		t.Errorf("c.go = %v, want -1", cov["c.go"])
	}
}

func TestCommentBody(t *testing.T) {
	build := &core.Build{Number: 12, BaseBuildNumber: 9}
	verdict := &core.Verdict{State: core.StatusFailure, Description: "Coverage decreased 1.0% to 84.0%"}
	changes := []*core.FileChange{
		{Path: "a.go"},
		{Path: "c.go"},
		{Path: "gone.go", Deleted: true},
		{Path: "a|b.go"},
	}
	cov := map[string]float64{"a.go": 0.853, "c.go": -1, "a|b.go": 0.5}
	body := commentBody(build, verdict, changes, cov, "http://h/report/github/o/r/builds/12")

	for _, want := range []string{
		"## Coverage ❌ failed",
		"Coverage decreased 1.0% to 84.0%",
		"[Build #12](http://h/report/github/o/r/builds/12)",
		"base #9",
		"### Changed files",
		"| a.go | 85.3% |",
		"| c.go | — |",
		"| gone.go | deleted |",
		`| a\|b.go | 50.0% |`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s", want, body)
		}
	}
}

func TestCommentBodyNoChangesNoBase(t *testing.T) {
	build := &core.Build{Number: 3, BaseBuildNumber: 0}
	verdict := &core.Verdict{State: core.StatusSuccess, Description: "Coverage: 90.0%"}
	body := commentBody(build, verdict, nil, map[string]float64{}, "http://h/x")

	if !strings.Contains(body, "## Coverage ✅ passed") {
		t.Errorf("missing success header:\n%s", body)
	}
	if strings.Contains(body, "base #") {
		t.Errorf("should not mention base when BaseBuildNumber is 0:\n%s", body)
	}
	if strings.Contains(body, "### Changed files") {
		t.Errorf("should not render changed-files section when there are no changes:\n%s", body)
	}
}
