package scm

import (
	"testing"

	core "github.com/covergates/covergates/core"
	scm "github.com/drone/go-scm/scm"
)

func TestNewReportID(t *testing.T) {
	s := &repoService{}
	reportID1 := s.NewReportID(&core.Repo{
		URL: "http://repo",
	})
	reportID2 := s.NewReportID(&core.Repo{
		URL: "http://repo",
	})
	if reportID1 == reportID2 {
		t.Logf("%s %s", reportID1, reportID2)
		t.Fail()
	}
}

func TestToSCMState(t *testing.T) {
	cases := []struct {
		in   core.StatusState
		want scm.State
	}{
		{core.StatusSuccess, scm.StateSuccess},
		{core.StatusFailure, scm.StateFailure},
		{core.StatusError, scm.StateError},
		{core.StatusPending, scm.StatePending},
		{core.StatusState("garbage"), scm.StatePending},
	}
	for _, tc := range cases {
		if got := toSCMState(tc.in); got != tc.want {
			t.Fatalf("toSCMState(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
