// internal/router/scorer.go
//
// Scorer 对单个后端进行多维度加权打分。
// 分数越低 = 后端越适合接请求。
// 0 分 = 完美后端（空闲的 Healthy A100 且延迟为 0）。

package router

import (
	"math"

	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
)

// Scorer 对后端进行多维度打分。
type Scorer struct {
	weights ScoringWeights
}

// NewScorer 创建打分器。
func NewScorer() *Scorer {
	return &Scorer{weights: DefaultWeights()}
}

// Score 对单个后端打分。
// 返回 0-100 的分数。分数越低越好。
// Unhealthy 后端返回 MaxFloat64。
func (s *Scorer) Score(backend BackendSnapshot) float64 {
	// 1. Unhealthy 直接淘汰
	if backend.HealthState == health.Unhealthy.String() {
		return math.MaxFloat64
	}

	// 2. Latency 维度（0-100）
	// LastLatency 是秒，×100 映射到 0-100
	// 例如 0.05s = 5 分，0.5s = 50 分，2s = 100 分（封顶）
	latencyScore := math.Min(backend.LastLatency*100, 100)

	// 3. Queue Depth 维度（0-100）
	// 使用率 = 当前在飞 / 最大并发
	// 使用率 0% = 0 分，100% = 100 分
	var queueScore float64
	if backend.MaxRequests > 0 {
		usageRatio := float64(backend.ActiveRequests) / float64(backend.MaxRequests)
		queueScore = usageRatio * 100
	}

	// 4. Health 维度
	// Healthy = 0 分，Suspect = 50 分
	// 注意：Unhealthy 在步骤 1 已经淘汰，不会走到这里
	var healthScore float64
	if backend.HealthState == health.Suspect.String() {
		healthScore = 50
	}

	// 5. GPU 维度
	// A100 = 0 分，A10 = 20 分，未知 = 10 分
	var gpuScore float64
	switch backend.GPUType {
	case "A100-80G", "A100-40G":
		gpuScore = 0
	case "A10-24G":
		gpuScore = 20
	default:
		gpuScore = 10
	}

	// 6. 加权求和
	total := s.weights.Latency*latencyScore +
		s.weights.QueueDepth*queueScore +
		s.weights.Health*healthScore +
		s.weights.GPU*gpuScore

	return total
}
