package workspace

import (
	"testing"
)

func TestFindRoot(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Fatalf("FindRoot() returned error: %v", err)
	}
	if root == "" {
		t.Fatal("FindRoot() returned empty string")
	}
	// Verify it's a valid path
	if root[0] != '/' {
		t.Errorf("FindRoot() returned non-absolute path: %s", root)
	}
}
