package lca

import (
	"path/filepath"
	"strings"
)

// FindLCA finds the least common ancestor directory of the given file paths.
func FindLCA(files []string) string {
	if len(files) == 0 {
		return "."
	}

	// Get directory of each file
	dirs := make([][]string, len(files))
	for i, f := range files {
		dir := filepath.Dir(f)
		if dir == "." {
			dirs[i] = []string{}
		} else {
			dirs[i] = strings.Split(filepath.ToSlash(dir), "/")
		}
	}

	if len(dirs[0]) == 0 {
		return "."
	}

	// Find common prefix
	result := []string{}
	for i := 0; i < len(dirs[0]); i++ {
		component := dirs[0][i]
		allMatch := true
		for _, d := range dirs[1:] {
			if i >= len(d) || d[i] != component {
				allMatch = false
				break
			}
		}
		if !allMatch {
			break
		}
		result = append(result, component)
	}

	if len(result) == 0 {
		return "."
	}
	return strings.Join(result, "/")
}
