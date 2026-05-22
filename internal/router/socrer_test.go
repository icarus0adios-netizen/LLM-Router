// internal/router/scorer_test.go

package router

import (
	"math"
	"testing"

	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
)

func TestScorer_UnhealthyReturnsMax(t *testing.T) {
	s := NewScorer()

	snapshot := BackendSnapshot{
		ID:          "dead-backend",
		HealthState: health.Unhealthy.String(),
	}
	score := s.Score(snapshot)

	if score != math.MaxFloat64 {
		t.Errorf("Unhealthy backend: got %.2f, want MaxFloat64", score)
	}
}

func TestScorer_HealthyBeatsSuspect(t *testing.T) {
	s := NewScorer()

	// 两个后端其他条件完全相同，只有健康状态不同
	healthy := BackendSnapshot{
		ID: "h", URL: "http://h:9001",
		HealthState:    health.Healthy.String(),
		LastLatency:    0.01,
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}
	suspect := BackendSnapshot{
		ID: "s", URL: "http://s:9002",
		HealthState:    health.Suspect.String(),
		LastLatency:    0.01,
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}

	healthyScore := s.Score(healthy)
	suspectScore := s.Score(suspect)

	if healthyScore >= suspectScore {
		t.Errorf("Healthy should score lower (better) than Suspect. healthy=%.2f, suspect=%.2f",
			healthyScore, suspectScore)
	}
}

func TestScorer_IdleBeatsBusy(t *testing.T) {
	s := NewScorer()

	idle := BackendSnapshot{
		ID:             "idle",
		HealthState:    health.Healthy.String(),
		LastLatency:    0.01,
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}
	busy := BackendSnapshot{
		ID:             "busy",
		HealthState:    health.Healthy.String(),
		LastLatency:    0.01,
		ActiveRequests: 8, MaxRequests: 10,
		GPUType: "A100-80G",
	}

	idleScore := s.Score(idle)
	busyScore := s.Score(busy)

	if idleScore >= busyScore {
		t.Errorf("Idle backend (%.2f) should score lower than busy (%.2f)", idleScore, busyScore)
	}
}

func TestScorer_A100BeatsA10(t *testing.T) {
	s := NewScorer()

	a100 := BackendSnapshot{
		ID: "a100", HealthState: health.Healthy.String(),
		LastLatency:    0.01,
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}
	a10 := BackendSnapshot{
		ID: "a10", HealthState: health.Healthy.String(),
		LastLatency:    0.01,
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A10-24G",
	}

	a100Score := s.Score(a100)
	a10Score := s.Score(a10)

	if a100Score >= a10Score {
		t.Errorf("A100 (%.2f) should score lower than A10 (%.2f) when all else equal",
			a100Score, a10Score)
	}
}

func TestScorer_LowLatencyBeatsHighLatency(t *testing.T) {
	s := NewScorer()

	fast := BackendSnapshot{
		ID: "fast", HealthState: health.Healthy.String(),
		LastLatency:    0.01, // 10ms
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}
	slow := BackendSnapshot{
		ID: "slow", HealthState: health.Healthy.String(),
		LastLatency:    0.50, // 500ms
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}

	fastScore := s.Score(fast)
	slowScore := s.Score(slow)

	if fastScore >= slowScore {
		t.Errorf("Low latency (%.2f) should score lower than high latency (%.2f)", fastScore, slowScore)
	}
}

func TestScorer_QueueDepthDominates(t *testing.T) {
	// 这个测试验证 QueueDepth 权重（30%）在实际场景中的效果
	s := NewScorer()

	// 一个稍有延迟但空闲的 A100
	idleSlow := BackendSnapshot{
		ID: "idle-slow", HealthState: health.Healthy.String(),
		LastLatency:    0.10, // 100ms - 稍慢
		ActiveRequests: 0, MaxRequests: 10,
		GPUType: "A100-80G",
	}

	// 一个很快但满的 A100
	busyFast := BackendSnapshot{
		ID: "busy-fast", HealthState: health.Healthy.String(),
		LastLatency:    0.01,               // 10ms - 很快
		ActiveRequests: 9, MaxRequests: 10, // 几乎满了
		GPUType: "A100-80G",
	}

	idleSlowScore := s.Score(idleSlow)
	busyFastScore := s.Score(busyFast)

	// idleSlow: 0.40×10 + 0.30×0 = 4.0
	// busyFast: 0.40×1  + 0.30×90 = 27.4
	// idleSlow 应该胜出

	if idleSlowScore >= busyFastScore {
		t.Errorf("Idle-slow (%.2f) should beat busy-fast (%.2f): queue depth matters",
			idleSlowScore, busyFastScore)
	}
}
