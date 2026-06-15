package util

import "testing"

func TestGenerateToken(t *testing.T) {
	a := GenerateToken()
	b := GenerateToken()
	if a == "" {
		t.Fatal("token should not be empty")
	}
	if len(a) != 32 {
		t.Fatalf("token length = %d, want 32", len(a))
	}
	if a == b {
		t.Fatal("two generated tokens should differ")
	}
}
