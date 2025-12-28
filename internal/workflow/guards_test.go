package workflow

import (
	"context"
	"testing"
)

func TestGuardHasSource(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "nil work unit",
			wu:   nil,
			want: false,
		},
		{
			name: "nil source",
			wu:   &WorkUnit{ID: "test"},
			want: false,
		},
		{
			name: "empty reference",
			wu:   &WorkUnit{ID: "test", Source: &Source{}},
			want: false,
		},
		{
			name: "valid source",
			wu:   &WorkUnit{ID: "test", Source: &Source{Reference: "file:task.md"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardHasSource(ctx, tt.wu); got != tt.want {
				t.Errorf("GuardHasSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardNoSpecifications(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "nil work unit",
			wu:   nil,
			want: false,
		},
		{
			name: "empty specifications",
			wu:   &WorkUnit{ID: "test", Specifications: []string{}},
			want: true,
		},
		{
			name: "nil specifications",
			wu:   &WorkUnit{ID: "test"},
			want: true,
		},
		{
			name: "has specifications",
			wu:   &WorkUnit{ID: "test", Specifications: []string{"specification-1.md"}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardNoSpecifications(ctx, tt.wu); got != tt.want {
				t.Errorf("GuardNoSpecifications() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardHasSpecifications(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "nil work unit",
			wu:   nil,
			want: false,
		},
		{
			name: "empty specifications",
			wu:   &WorkUnit{ID: "test", Specifications: []string{}},
			want: false,
		},
		{
			name: "has specifications",
			wu:   &WorkUnit{ID: "test", Specifications: []string{"specification-1.md"}},
			want: true,
		},
		{
			name: "multiple specifications",
			wu:   &WorkUnit{ID: "test", Specifications: []string{"specification-1.md", "specification-2.md"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardHasSpecifications(ctx, tt.wu); got != tt.want {
				t.Errorf("GuardHasSpecifications() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanUndo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "nil work unit",
			wu:   nil,
			want: false,
		},
		{
			name: "no checkpoints",
			wu:   &WorkUnit{ID: "test"},
			want: false,
		},
		{
			name: "empty checkpoints",
			wu:   &WorkUnit{ID: "test", Checkpoints: []string{}},
			want: false,
		},
		{
			name: "has checkpoints",
			wu:   &WorkUnit{ID: "test", Checkpoints: []string{"cp1", "cp2"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardCanUndo(ctx, tt.wu); got != tt.want {
				t.Errorf("GuardCanUndo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanRedo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "nil work unit",
			wu:   nil,
			want: false,
		},
		{
			name: "work unit exists",
			wu:   &WorkUnit{ID: "test"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardCanRedo(ctx, tt.wu); got != tt.want {
				t.Errorf("GuardCanRedo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanFinish(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "nil work unit",
			wu:   nil,
			want: false,
		},
		{
			name: "work unit exists without specifications",
			wu:   &WorkUnit{ID: "test"},
			want: false, // Changed: now requires specifications
		},
		{
			name: "work unit with specifications",
			wu:   &WorkUnit{ID: "test", Specifications: []string{"specification-1.md"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardCanFinish(ctx, tt.wu); got != tt.want {
				t.Errorf("GuardCanFinish() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateGuards(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		wu     *WorkUnit
		guards []GuardFunc
		want   bool
	}{
		{
			name:   "empty guards",
			wu:     &WorkUnit{ID: "test"},
			guards: []GuardFunc{},
			want:   true,
		},
		{
			name:   "nil guards",
			wu:     &WorkUnit{ID: "test"},
			guards: nil,
			want:   true,
		},
		{
			name:   "single passing guard",
			wu:     &WorkUnit{ID: "test", Source: &Source{Reference: "f"}},
			guards: []GuardFunc{GuardHasSource},
			want:   true,
		},
		{
			name:   "single failing guard",
			wu:     &WorkUnit{ID: "test"},
			guards: []GuardFunc{GuardHasSource},
			want:   false,
		},
		{
			name: "multiple guards all pass",
			wu: &WorkUnit{
				ID:     "test",
				Source: &Source{Reference: "f"},
			},
			guards: []GuardFunc{GuardHasSource, GuardNoSpecifications},
			want:   true,
		},
		{
			name: "multiple guards one fails",
			wu: &WorkUnit{
				ID:             "test",
				Source:         &Source{Reference: "f"},
				Specifications: []string{"specification-1.md"},
			},
			guards: []GuardFunc{GuardHasSource, GuardNoSpecifications},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EvaluateGuards(ctx, tt.wu, tt.guards); got != tt.want {
				t.Errorf("EvaluateGuards() = %v, want %v", got, tt.want)
			}
		})
	}
}
