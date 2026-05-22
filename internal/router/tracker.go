// internal/router/tracker.go
//
// RequestTracker 追踪每个后端的在飞请求数。
// 用于 Scorer 的 QueueDepth 维度打分。
// 线程安全。

package router

import (
	"sync"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
)

// RequestTracker 追踪每个后端的在飞请求数。
// 在请求分发前 Inc，请求完成后 Dec。
type RequestTracker struct {
	mu       sync.Mutex
	counters map[string]int32 // backendID → 当前在飞请求数
	maxReqs  map[string]int32 // backendID → 最大并发数
}

// NewRequestTracker 创建请求追踪器。
// 从 config 读取每个后端的最大并发数（先用默认值 10）。
func NewRequestTracker(backends []config.BackendConfig) *RequestTracker {
	t := &RequestTracker{
		counters: make(map[string]int32, len(backends)),
		maxReqs:  make(map[string]int32, len(backends)),
	}
	for _, b := range backends {
		t.counters[b.ID] = 0
		t.maxReqs[b.ID] = 10 // 默认每个后端最大 10 并发，后续可配置
	}
	return t
}

// Inc 增加指定后端的在飞请求数。返回增加后的值。
func (t *RequestTracker) Inc(backendID string) int32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counters[backendID]++
	return t.counters[backendID]
}

// Dec 减少指定后端的在飞请求数。必须在请求完成后调用。
// 用 defer 确保即使 panic 也会执行。
func (t *RequestTracker) Dec(backendID string) int32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.counters[backendID] > 0 {
		t.counters[backendID]--
	}
	return t.counters[backendID]
}

// Snapshot 返回当前所有后端的负载信息。
// 返回 map[backendID] → {ActiveRequests, MaxRequests}
// 调用方拿到的是副本，不会与内部 map 产生数据竞争。
func (t *RequestTracker) Snapshot() map[string]ActiveRequests {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string]ActiveRequests, len(t.counters))
	for id, count := range t.counters {
		result[id] = ActiveRequests{
			Active: int(count),
			Max:    int(t.maxReqs[id]),
		}
	}
	return result
}
