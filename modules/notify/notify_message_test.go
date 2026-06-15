package notify

import (
	"strings"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

func TestShouldNotify(t *testing.T) {
	fail := &core.Verdict{State: core.StatusFailure}
	errd := &core.Verdict{State: core.StatusError}
	ok := &core.Verdict{State: core.StatusSuccess}
	cases := []struct {
		trigger string
		verdict *core.Verdict
		want    bool
	}{
		{"off", fail, false},
		{"always", ok, true},
		{"always", fail, true},
		{"failure", ok, false},
		{"failure", fail, true},
		{"failure", errd, true},
		{"", ok, false},  // empty -> failure default
		{"", fail, true}, // empty -> failure default
	}
	for _, tc := range cases {
		if got := shouldNotify(tc.trigger, tc.verdict); got != tc.want {
			t.Errorf("shouldNotify(%q, %v) = %v, want %v", tc.trigger, tc.verdict.State, got, tc.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	if statusLabel(core.StatusSuccess) != "passed" {
		t.Error("success should be passed")
	}
	if statusLabel(core.StatusFailure) != "failed" {
		t.Error("failure should be failed")
	}
	if statusLabel(core.StatusError) != "errored" {
		t.Error("error should be errored")
	}
	if statusLabel(core.StatusPending) != "pending" {
		t.Error("pending should be pending")
	}
}

func TestSummaryLineAndBuildURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	repo := &core.Repo{NameSpace: "o", Name: "r", SCM: core.Github}
	build := &core.Build{Number: 12}
	verdict := &core.Verdict{State: core.StatusFailure, Description: "Coverage decreased 1.0% to 84.0%"}

	line := summaryLine(repo, build, verdict)
	for _, want := range []string{"o/r", "#12", "failed", "Coverage decreased 1.0% to 84.0%"} {
		if !strings.Contains(line, want) {
			t.Errorf("summaryLine %q missing %q", line, want)
		}
	}
	if url := buildURL(cfg, repo, build); url != "http://localhost:8080/report/github/o/r/builds/12" {
		t.Errorf("buildURL = %q", url)
	}
}
