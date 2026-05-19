// internal/router/router.go
//
// Router 实现 Filter→Score→Select 路由决策链。
// 这是整个网关的核心调度逻辑。
package router

import (
	"fmt"
	"math"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
	"github.com/icarus0adios-netizen/LLM-Router/internal/metrics"
)

// Router 执行 Filter→Score→Select 路由决策。
//
// 依赖注入：
//   - checker: 提供后端健康状态
//   - tracker: 提供后端在飞请求数
//   - scorer:  多维度加权打分
//   - backends: 静态配置（URL、模型名、GPU 类型等）

type Router struct {
	checker  *health.Checker
	tracker  *RequestTracker
	scorer   *Scorer
	backends []config.BackendConfig
}

func NewRouter(checker *health.Checker,
	tracker *RequestTracker,
	scorer *Scorer,
	backends []config.BackendConfig) *Router {
	return &Router{
		checker:  checker,
		tracker:  tracker,
		scorer:   scorer,
		backends: backends,
	}
}

// Route 执行 Filter→Score→Select，返回最优后端 URL。
//
// 流程：
//  1. 从 checker 获取所有后端健康状态
//  2. 从 tracker 获取所有后端负载快照
//  3. Filter: 淘汰 Unhealthy + 满负载后端
//  4. 为每个候选构造 BackendSnapshot
//  5. Score: 对每个候选打分
//  6. Select: 选最低分
//  7. 返回 winner 的 URL
//
// 如果没有可用后端，返回 error。
func (r *Router) Route() (string, string, error) { //Backend ID , URL ,Error
	states := r.checker.States()
	if len(states) == 0 {
		return "", "", fmt.Errorf("no healthy backends")
	}

	snapshots := r.tracker.Snapshot()
	if len(snapshots) == 0 {
		return "", "", fmt.Errorf("no healthy backends")
	}

	backends := make(map[string]BackendSnapshot)

	//构造 BackendSnapshot
	for _, backend := range r.backends {
		//Filter: 淘汰 Unhealthy + 满负载后端
		if states[backend.ID] == health.Unhealthy.String() {
			continue
		}

		if snapshots[backend.ID].Active >= snapshots[backend.ID].Max {
			continue
		}

		backends[backend.ID] = BackendSnapshot{
			ID:             backend.ID,
			URL:            backend.URL,
			Model:          backend.Model,
			GPUType:        backend.GPUType,
			HealthState:    states[backend.ID],
			LastLatency:    r.checker.Latencies()[backend.ID],
			ActiveRequests: snapshots[backend.ID].Active,
			MaxRequests:    snapshots[backend.ID].Max,
		}
	}

	if len(backends) == 0 {
		return "", "", fmt.Errorf("no available backend")
	}

	targetID := ""
	targetScore := math.MaxFloat64
	for backendID, backend := range backends {
		score := r.scorer.Score(backend)
		if score < targetScore {
			targetID = backendID
			targetScore = score
		}
	}

	//记录metrics 后端score
	metrics.RecordRouterScore(targetID, targetScore)

	r.tracker.Inc(targetID)
	return targetID, backends[targetID].URL, nil
}
