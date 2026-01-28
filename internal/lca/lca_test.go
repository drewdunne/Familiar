package lca

import "testing"

func TestFindLCA_SingleFile(t *testing.T) {
	files := []string{"services/auth/handler.go"}
	result := FindLCA(files)
	if result != "services/auth" {
		t.Errorf("LCA = %q, want %q", result, "services/auth")
	}
}

func TestFindLCA_SameDirectory(t *testing.T) {
	files := []string{
		"services/auth/handler.go",
		"services/auth/utils.go",
	}
	result := FindLCA(files)
	if result != "services/auth" {
		t.Errorf("LCA = %q, want %q", result, "services/auth")
	}
}

func TestFindLCA_SiblingDirectories(t *testing.T) {
	files := []string{
		"services/auth/handler.go",
		"services/billing/handler.go",
	}
	result := FindLCA(files)
	if result != "services" {
		t.Errorf("LCA = %q, want %q", result, "services")
	}
}

func TestFindLCA_DifferentTrees(t *testing.T) {
	files := []string{
		"services/auth/handler.go",
		"lib/utils.go",
	}
	result := FindLCA(files)
	if result != "." {
		t.Errorf("LCA = %q, want %q", result, ".")
	}
}

func TestFindLCA_RootFiles(t *testing.T) {
	files := []string{"README.md", "go.mod"}
	result := FindLCA(files)
	if result != "." {
		t.Errorf("LCA = %q, want %q", result, ".")
	}
}

func TestFindLCA_Empty(t *testing.T) {
	result := FindLCA([]string{})
	if result != "." {
		t.Errorf("LCA = %q, want %q", result, ".")
	}
}
