package intent

import "testing"

func TestParsedIntent_HasAction(t *testing.T) {
	tests := []struct {
		name    string
		actions []Action
		check   Action
		want    bool
	}{
		{
			name:    "has action",
			actions: []Action{ActionMerge, ActionApprove},
			check:   ActionMerge,
			want:    true,
		},
		{
			name:    "does not have action",
			actions: []Action{ActionApprove},
			check:   ActionMerge,
			want:    false,
		},
		{
			name:    "empty actions",
			actions: []Action{},
			check:   ActionMerge,
			want:    false,
		},
		{
			name:    "nil actions",
			actions: nil,
			check:   ActionMerge,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ParsedIntent{
				RequestedActions: tt.actions,
			}
			if got := p.HasAction(tt.check); got != tt.want {
				t.Errorf("HasAction() = %v, want %v", got, tt.want)
			}
		})
	}
}
