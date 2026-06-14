package build

import "github.com/covergates/covergates/core"

// Merge combines source files from multiple jobs into one set. Files are unioned
// by name (insertion order preserved); per line, the result is nil when every
// job is nil there, otherwise the sum of non-nil hit counts. Inputs are not mutated.
func Merge(jobs [][]*core.SourceFile) []*core.SourceFile {
	merged := make(map[string]*core.SourceFile)
	order := make([]string, 0)
	for _, files := range jobs {
		for _, f := range files {
			existing, ok := merged[f.Name]
			if !ok {
				merged[f.Name] = &core.SourceFile{
					Name:         f.Name,
					SourceDigest: f.SourceDigest,
					Coverage:     cloneCoverage(f.Coverage),
				}
				order = append(order, f.Name)
				continue
			}
			existing.Coverage = mergeCoverage(existing.Coverage, f.Coverage)
			if existing.SourceDigest == "" {
				existing.SourceDigest = f.SourceDigest
			}
		}
	}
	result := make([]*core.SourceFile, len(order))
	for i, name := range order {
		result[i] = merged[name]
	}
	return result
}

// Coverage computes the statement coverage ratio (0..1) over source files.
func Coverage(files []*core.SourceFile) float64 {
	var relevant, covered int
	for _, f := range files {
		for _, hit := range f.Coverage {
			if hit == nil {
				continue
			}
			relevant++
			if *hit > 0 {
				covered++
			}
		}
	}
	if relevant == 0 {
		return 0
	}
	return float64(covered) / float64(relevant)
}

func cloneCoverage(c []*int) []*int {
	out := make([]*int, len(c))
	for i, hit := range c {
		if hit != nil {
			v := *hit
			out[i] = &v
		}
	}
	return out
}

func mergeCoverage(a, b []*int) []*int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	out := make([]*int, n)
	for i := 0; i < n; i++ {
		var pa, pb *int
		if i < len(a) {
			pa = a[i]
		}
		if i < len(b) {
			pb = b[i]
		}
		if pa == nil && pb == nil {
			continue
		}
		sum := 0
		if pa != nil {
			sum += *pa
		}
		if pb != nil {
			sum += *pb
		}
		out[i] = &sum
	}
	return out
}
