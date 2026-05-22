package router

import (
	"fmt"
	"sync"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
)

// RouteSelector 路由接口，支持替换路由策略
type RouteSelector interface {
	Route() (string, string, error)
}

// RoundRobinRouter 轮询路由：过滤不健康后端，然后轮流选择
type RoundRobinRouter struct {
	mu       sync.Mutex
	current  int
	checker  *health.Checker
	tracker  *RequestTracker
	backends []config.BackendConfig
}

func NewRoundRobinRouter(checker *health.Checker, tracker *RequestTracker, backends []config.BackendConfig) *RoundRobinRouter {
	return &RoundRobinRouter{
		checker:  checker,
		tracker:  tracker,
		backends: backends,
	}
}

// Route 实现 RouteSelector 接口
func (r *RoundRobinRouter) Route() (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	states := r.checker.States()
	snapshots := r.tracker.Snapshot()

	var healthy []config.BackendConfig
	for _, b := range r.backends {
		if states[b.ID] == health.Unhealthy.String() {
			continue
		}
		if snapshots[b.ID].Active >= snapshots[b.ID].Max {
			continue
		}
		healthy = append(healthy, b)
	}

	if len(healthy) == 0 {
		return "", "", fmt.Errorf("no healthy backends")
	}

	b := healthy[r.current%len(healthy)]
	r.current++

	r.tracker.Inc(b.ID)
	return b.ID, b.URL, nil
}
