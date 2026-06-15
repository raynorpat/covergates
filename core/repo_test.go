package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRepoTokenNotSerialized guards against leaking the secret upload token
// through any JSON repo payload (notably the public GET /repos/:scm/:ns/:name).
func TestRepoTokenNotSerialized(t *testing.T) {
	data, err := json.Marshal(&Repo{
		Name:      "name",
		NameSpace: "space",
		Token:     "super-secret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "super-secret-token") {
		t.Fatalf("repo JSON leaked the token: %s", data)
	}
	if strings.Contains(string(data), "Token") {
		t.Fatalf("repo JSON exposed a Token field: %s", data)
	}
}
