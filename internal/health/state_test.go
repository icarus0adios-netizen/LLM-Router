package health

import (
	"testing"
)

func TestHealthState_String(t *testing.T) {
	tests := []struct {
		state HealthState
		want  string
	}{
		{Healthy, "healthy"},
		{Suspect, "suspect"},
		{Unhealthy, "unhealthy"},
		{HealthState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("HealthState(%d).String() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestHealthState_IsHealthy(t *testing.T) {
	tests := []struct {
		state HealthState
		want  bool
	}{
		{Healthy, true},
		{Suspect, true},
		{Unhealthy, false},
	}
	for _, tt := range tests {
		if got := tt.state.IsHealthy(); got != tt.want {
			t.Errorf("HealthState(%d).IsHealthy() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestHealthState_IsUnhealthy(t *testing.T) {
	tests := []struct {
		state HealthState
		want  bool
	}{
		{Healthy, false},
		{Suspect, false},
		{Unhealthy, true},
	}
	for _, tt := range tests {
		if got := tt.state.IsUnhealthy(); got != tt.want {
			t.Errorf("HealthState(%d).IsUnhealthy() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestTransitionState_HealthyState(t *testing.T) {
	tests := []struct {
		name                string
		successes           int
		fails               int
		success             bool
		wantState           HealthState
		wantSuccesses       int
		wantFails           int
	}{
		{
			name:      "Healthy: 单次成功，保持 Healthy",
			successes: 0,
			fails:     0,
			success:   true,
			wantState: Healthy,
			wantSuccesses: 1,
			wantFails: 0,
		},
		{
			name:      "Healthy: 1次失败，保持 Healthy",
			successes: 0,
			fails:     0,
			success:   false,
			wantState: Healthy,
			wantSuccesses: 0,
			wantFails: 1,
		},
		{
			name:      "Healthy: 2次失败，保持 Healthy",
			successes: 0,
			fails:     1,
			success:   false,
			wantState: Healthy,
			wantSuccesses: 0,
			wantFails: 2,
		},
		{
			name:      "Healthy: 3次失败，降级为 Suspect",
			successes: 0,
			fails:     2,
			success:   false,
			wantState: Suspect,
			wantSuccesses: 0,
			wantFails: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotSuccesses, gotFails, err := TransitionState(Healthy, tt.successes, tt.fails, tt.success)
			if err != nil {
				t.Errorf("TransitionState() error = %v", err)
			}
			if gotState != tt.wantState {
				t.Errorf("TransitionState() state = %v, want %v", gotState, tt.wantState)
			}
			if gotSuccesses != tt.wantSuccesses {
				t.Errorf("TransitionState() successes = %v, want %v", gotSuccesses, tt.wantSuccesses)
			}
			if gotFails != tt.wantFails {
				t.Errorf("TransitionState() fails = %v, want %v", gotFails, tt.wantFails)
			}
		})
	}
}

func TestTransitionState_SuspectState(t *testing.T) {
	tests := []struct {
		name                string
		successes           int
		fails               int
		success             bool
		wantState           HealthState
		wantSuccesses       int
		wantFails           int
	}{
		{
			name:      "Suspect: 连续成功 < 2，保持 Suspect",
			successes: 0,
			fails:     0,
			success:   true,
			wantState: Suspect,
			wantSuccesses: 1,
			wantFails: 0,
		},
		{
			name:      "Suspect: 连续成功 >= 2，恢复 Healthy",
			successes: 1,
			fails:     0,
			success:   true,
			wantState: Healthy,
			wantSuccesses: 2,
			wantFails: 0,
		},
		{
			name:      "Suspect: 连续失败 < 2，保持 Suspect",
			successes: 0,
			fails:     0,
			success:   false,
			wantState: Suspect,
			wantSuccesses: 0,
			wantFails: 1,
		},
		{
			name:      "Suspect: 连续失败 >= 2，降级 Unhealthy",
			successes: 0,
			fails:     1,
			success:   false,
			wantState: Unhealthy,
			wantSuccesses: 0,
			wantFails: 2,
		},
		{
			name:      "Suspect: 成功一次后重置失败计数",
			successes: 0,
			fails:     1,
			success:   true,
			wantState: Suspect,
			wantSuccesses: 1,
			wantFails: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotSuccesses, gotFails, err := TransitionState(Suspect, tt.successes, tt.fails, tt.success)
			if err != nil {
				t.Errorf("TransitionState() error = %v", err)
			}
			if gotState != tt.wantState {
				t.Errorf("TransitionState() state = %v, want %v", gotState, tt.wantState)
			}
			if gotSuccesses != tt.wantSuccesses {
				t.Errorf("TransitionState() successes = %v, want %v", gotSuccesses, tt.wantSuccesses)
			}
			if gotFails != tt.wantFails {
				t.Errorf("TransitionState() fails = %v, want %v", gotFails, tt.wantFails)
			}
		})
	}
}

func TestTransitionState_UnhealthyState(t *testing.T) {
	tests := []struct {
		name                string
		successes           int
		fails               int
		success             bool
		wantState           HealthState
		wantSuccesses       int
		wantFails           int
	}{
		{
			name:      "Unhealthy: 继续失败，保持 Unhealthy",
			successes: 0,
			fails:     0,
			success:   false,
			wantState: Unhealthy,
			wantSuccesses: 0,
			wantFails: 1,
		},
		{
			name:      "Unhealthy: 1次成功，保持 Unhealthy",
			successes: 0,
			fails:     0,
			success:   true,
			wantState: Unhealthy,
			wantSuccesses: 1,
			wantFails: 0,
		},
		{
			name:      "Unhealthy: 2次成功，保持 Unhealthy",
			successes: 1,
			fails:     0,
			success:   true,
			wantState: Unhealthy,
			wantSuccesses: 2,
			wantFails: 0,
		},
		{
			name:      "Unhealthy: 3次成功，恢复 Suspect",
			successes: 2,
			fails:     0,
			success:   true,
			wantState: Suspect,
			wantSuccesses: 3,
			wantFails: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotSuccesses, gotFails, err := TransitionState(Unhealthy, tt.successes, tt.fails, tt.success)
			if err != nil {
				t.Errorf("TransitionState() error = %v", err)
			}
			if gotState != tt.wantState {
				t.Errorf("TransitionState() state = %v, want %v", gotState, tt.wantState)
			}
			if gotSuccesses != tt.wantSuccesses {
				t.Errorf("TransitionState() successes = %v, want %v", gotSuccesses, tt.wantSuccesses)
			}
			if gotFails != tt.wantFails {
				t.Errorf("TransitionState() fails = %v, want %v", gotFails, tt.wantFails)
			}
		})
	}
}

func TestTransitionState_ErrorState(t *testing.T) {
	_, _, _, err := TransitionState(HealthState(99), 0, 0, true)
	if err == nil {
		t.Error("TransitionState() with unknown state should return error")
	}
}
