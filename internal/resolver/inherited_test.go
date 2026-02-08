package resolver

import "testing"

const (
	taskMyTask  = "my-task"
	taskMyTask1 = "my-task-1"
)

func TestIsInherited(t *testing.T) {
	tests := []struct {
		name     string
		task     string
		allTasks []string
		want     bool
	}{
		{
			name:     "duplicate with numeric suffix",
			task:     taskMyTask1,
			allTasks: []string{taskMyTask, taskMyTask1},
			want:     true,
		},
		{
			name:     "duplicate suffix 2",
			task:     "auth-2",
			allTasks: []string{"auth", "auth-1", "auth-2"},
			want:     true,
		},
		{
			name:     "not a duplicate original",
			task:     taskMyTask,
			allTasks: []string{taskMyTask, taskMyTask1},
			want:     false,
		},
		{
			name:     "non-numeric suffix",
			task:     "my-task-abc",
			allTasks: []string{taskMyTask, "my-task-abc"},
			want:     false,
		},
		{
			name:     "no base exists",
			task:     "orphan-1",
			allTasks: []string{"orphan-1", "other-task"},
			want:     false,
		},
		{
			name:     "no dash",
			task:     "single",
			allTasks: []string{"single"},
			want:     false,
		},
		{
			name:     "empty task list",
			task:     taskMyTask1,
			allTasks: nil,
			want:     false,
		},
		{
			name:     "dash at start",
			task:     "-1",
			allTasks: []string{"", "-1"},
			want:     false,
		},
		{
			name:     "task with multiple dashes",
			task:     "my-cool-task-3",
			allTasks: []string{"my-cool-task", "my-cool-task-3"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInherited(tt.task, tt.allTasks)
			if got != tt.want {
				t.Errorf("IsInherited(%q, %v) = %v, want %v", tt.task, tt.allTasks, got, tt.want)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"", false},
		{"12a3", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNumeric(tt.input)
			if got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
