// 包 metrics 定义并注册所有 Prometheus 指标。
// 全局单例模式：每个指标在 init 或 MustRegister 中注册一次，
// 业务代码直接调用 Inc() / Observe() 等方法来记录。

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	//1. ----------流量------------
	//Counter : 只增不减，适合统计QPS
	//Labels  : backend_id （后端标识） ， status (http状态码)
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_request_total",
			Help: "收到的请求总数，按后端和状态码分组。",
		},
		[]string{"backend_id", "status"},
	)

	//2. ----------延迟------------
	// Histogram: 自动计算 P50/P90/P99
	// Buckets 选择: 对 LLM 推理而言，TTFT 是关键指标
	// 0.05s=50ms（理想） → 0.5s → 2s → 10s（超时前）
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "gateway_request_duration_seconds",
			Help: "请求延迟分布，单位秒。",
		},
		[]string{"backend_id"},
	)

	// ---------- 3. 拒绝/失败计数 ----------
	// 按原因分 label: "admission_rejected", "no_backend", "backend_timeout", "backend_error"
	GatewayErrorTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_error_total",
			Help: "路由错误总数，按错误类型分组。",
		},
		[]string{"reason"},
	)

	// ---------- 4. 后端健康状态 ----------
	// Gauge: 可升可降，适合瞬时状态
	// 值: 0=Healthy, 1=Suspect, 2=Unhealthy
	BackendHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_backend_health",
			Help: "后端健康状态，0=Healthy, 1=Suspect, 2=Unhealthy",
		},
		[]string{"backend_id"},
	)

	// ---------- 5. 后端在飞请求数 ----------
	ActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_active_requests",
			Help: "后端当前在飞请求数",
		},
		[]string{"backend_id"},
	)

	// ---------- 6. 路由：选中的后端分数 ----------
	// Histogram: 观察路由打分分布，分数异常升高 → 所有后端都忙/慢
	RouterScore = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "gateway_router_score",
			Help: "Router 后端选中的分数，分数越低越好",
		},
		[]string{"backend_id"},
	)
)

// RecordRequest 在请求完成时记录指标。
// 调用方只需在请求结束时调一次。
func RecordRequest(BackendID string, statusCode int, durationSeconds float64) {
	statusLabel := "success"
	if statusCode >= 400 {
		statusLabel = "error"
	}
	RequestsTotal.WithLabelValues(BackendID, statusLabel).Inc()
	RequestDuration.WithLabelValues(BackendID).Observe(durationSeconds)
}

// RecordError 记录网关层面的错误（非后端错误）。
// reason: "admission_rejected", "no_backend", "backend_timeout", "backend_error"
func RecordError(reason string) {
	GatewayErrorTotal.WithLabelValues(reason).Inc()
}

// SetBackendHealth 更新后端健康状态 Gauge。
func SetBackendHealth(BackendID string, healthStatus float64) {
	BackendHealth.WithLabelValues(BackendID).Set(healthStatus)
}

// SetBackendActiveRequests 更新后端在飞请求数 Gauge。
func SetBackendActiveRequests(BackendID string, activeConns float64) {
	ActiveRequests.WithLabelValues(BackendID).Set(activeConns)
}

// RecordRouterScore 记录路由打分。
func RecordRouterScore(BackendID string, score float64) {
	RouterScore.WithLabelValues(BackendID).Observe(score)
}
