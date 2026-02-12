package instructions

import (
	"strings"
	"testing"
)

func TestContent_NonEmpty(t *testing.T) {
	content := Content()
	if content == "" {
		t.Fatal("Content() returned empty string")
	}
}

func TestContent_RequiredSections(t *testing.T) {
	content := Content()

	requiredSections := []string{
		"# Familiar Agent",
		"## Environment",
		"## Workflow",
		"## Concurrency",
		"## Constraints",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("Content() missing required section %q", section)
		}
	}
}

func TestContent_CriticalInstructions(t *testing.T) {
	content := Content()

	criticalPhrases := []struct {
		name   string
		phrase string
	}{
		{"never force-push", "force-push"},
		{"always push changes", "always push"},
		{"rebase workflow", "rebase"},
	}

	for _, tc := range criticalPhrases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(strings.ToLower(content), strings.ToLower(tc.phrase)) {
				t.Errorf("Content() missing critical instruction about %q", tc.name)
			}
		})
	}
}
