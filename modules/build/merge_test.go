package build

import (
	"testing"

	"github.com/covergates/covergates/core"
)

func p(v int) *int { return &v }

func TestMergeSumsHitsElementwise(t *testing.T) {
	jobA := []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{p(1), nil, p(0)}},
	}
	jobB := []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{p(2), nil, p(3)}},
		{Name: "b.go", Coverage: []*int{p(0)}},
	}

	merged := Merge([][]*core.SourceFile{jobA, jobB})

	if len(merged) != 2 {
		t.Fatalf("got %d files, want 2", len(merged))
	}
	a := merged[0]
	if a.Name != "a.go" {
		t.Fatalf("first file = %s, want a.go (insertion order)", a.Name)
	}
	if *a.Coverage[0] != 3 {
		t.Fatalf("line 1 = %d, want 3", *a.Coverage[0])
	}
	if a.Coverage[1] != nil {
		t.Fatal("line 2 should stay nil")
	}
	if *a.Coverage[2] != 3 {
		t.Fatalf("line 3 = %d, want 3", *a.Coverage[2])
	}
}

func TestMergeDoesNotMutateInput(t *testing.T) {
	jobA := []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(1)}}}
	jobB := []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(2)}}}
	Merge([][]*core.SourceFile{jobA, jobB})
	if *jobA[0].Coverage[0] != 1 {
		t.Fatal("Merge mutated its input")
	}
}

func TestCoverageRatio(t *testing.T) {
	files := []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{p(1), nil, p(0), p(2)}}, // 3 relevant, 2 covered
	}
	got := Coverage(files)
	want := 2.0 / 3.0
	if got != want {
		t.Fatalf("coverage = %v, want %v", got, want)
	}
}

func TestCoverageEmpty(t *testing.T) {
	if got := Coverage(nil); got != 0 {
		t.Fatalf("coverage of nil = %v, want 0", got)
	}
}
