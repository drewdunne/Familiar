package event

import (
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	debounceWindow := 100 * time.Millisecond
	d := NewDebouncer(debounceWindow)

	event1 := &Event{
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		Type:      TypeMRUpdated,
		MRNumber:  42,
	}

	// First event should be accepted
	if !d.ShouldProcess(event1) {
		t.Error("First event should be accepted")
	}

	// Same event immediately after should be debounced
	if d.ShouldProcess(event1) {
		t.Error("Duplicate event should be debounced")
	}

	// Wait for debounce window
	time.Sleep(debounceWindow + 10*time.Millisecond)

	// Now it should be accepted again
	if !d.ShouldProcess(event1) {
		t.Error("Event after debounce window should be accepted")
	}
}

func TestDebouncer_DifferentEvents(t *testing.T) {
	d := NewDebouncer(100 * time.Millisecond)

	event1 := &Event{
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		Type:      TypeMRUpdated,
		MRNumber:  42,
	}

	event2 := &Event{
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		Type:      TypeMRUpdated,
		MRNumber:  43, // Different MR
	}

	d.ShouldProcess(event1)

	// Different event should be accepted
	if !d.ShouldProcess(event2) {
		t.Error("Different event should be accepted")
	}
}
