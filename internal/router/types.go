package router

type BackendSnapshot struct {
	ID             string  // 后端标识
	URL            string  // 后端地址
	Model          string  // 模型名称
	GPUType        string  // GPU 类型（A100-80G / A10-24G 等）
	HealthState    string  // 健康状态
	LastLatency    float64 // 最近一次 /health 探测延迟（秒）
	ActiveRequests int     // 当前正在该后端处理的请求数（来自 RequestTracker）
	MaxRequests    int     // 该后端允许的最大并发数（来自 config）
}

// ScoringWeights 定义多维度打分的权重配置。
// 四个权重之和通常为 1.0。
type ScoringWeights struct {
	Latency    float64 // 最近延迟权重
	QueueDepth float64 // 排队深度权重
	Health     float64 // 健康状态权重
	GPU        float64 // GPU 类型权重
}

// ActiveRequests 描述单个后端的负载情况。
type ActiveRequests struct {
	Active int // 当前在飞请求数
	Max    int // 该后端最大并发数
}

// DefaultWeights 返回默认权重。
func DefaultWeights() ScoringWeights {
	return ScoringWeights{
		Latency:    0.40,
		QueueDepth: 0.30,
		Health:     0.20,
		GPU:        0.10,
	}
}
