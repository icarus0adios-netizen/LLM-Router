package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
)

type Checker struct {
	backends map[string]*BackendHealth // backend ID -> health state
	interval time.Duration             // 检查间隔
	client   *http.Client              // HTTP 客户端(复用连接，不重复创建)

	mu sync.RWMutex // 读写锁
}

// NewChecker 初始化健康检查器
// 初始化所有后端为 Suspect 状态（"无罪推定"，先试探再信任）
func NewChecker(cfg *config.Config, interval time.Duration) *Checker {
	checker := &Checker{
		backends: make(map[string]*BackendHealth),
		interval: interval,
		client: &http.Client{
			Timeout: 3 * time.Second, // 设置超时时间为 3 秒
		},
	}

	checker.mu.Lock()
	defer checker.mu.Unlock()
	for _, backend := range cfg.Backends {
		checker.backends[backend.ID] = &BackendHealth{
			ID:                 backend.ID,
			Addr:               backend.URL,
			State:              Suspect,
			ConsecutiveSuccess: 0,
			ConsecutiveFail:    0,
			LastCheck:          time.Now(),
			LastLatency:        0.0,
		}
	}
	return checker
}

func (c *Checker) checkOnce(backendID string) {
	// 1. 获取 backend（读锁）
	c.mu.RLock()
	backend, exists := c.backends[backendID] //注意这里的 backend 是指针，后续的调用都是引用传递
	c.mu.RUnlock()

	if !exists {
		return
	}

	// 2. 发送 HTTP 请求（不在锁内，允许并发检查多个后端）
	start := time.Now()
	resp, err := c.client.Get(backend.Addr + "/health")
	latency := time.Since(start).Seconds()

	// 3. 处理结果并更新状态（写锁保护）
	c.mu.Lock()
	defer c.mu.Unlock()

	backend.LastCheck = time.Now()
	backend.LastLatency = latency

	if err != nil {
		// 网络错误/连接失败 - TransitionState 内部会管理计数器
		backend.State, backend.ConsecutiveSuccess, backend.ConsecutiveFail, _ = TransitionState(
			backend.State, backend.ConsecutiveSuccess, backend.ConsecutiveFail, false,
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// HTTP 错误（如 500, 503 等）- TransitionState 内部会管理计数器
		backend.State, backend.ConsecutiveSuccess, backend.ConsecutiveFail, _ = TransitionState(
			backend.State, backend.ConsecutiveSuccess, backend.ConsecutiveFail, false,
		)
	} else {
		// 成功（HTTP 200）- TransitionState 内部会管理计数器
		backend.State, backend.ConsecutiveSuccess, backend.ConsecutiveFail, _ = TransitionState(
			backend.State, backend.ConsecutiveSuccess, backend.ConsecutiveFail, true,
		)
	}
}

func (c *Checker) checkAll() {
	//先快照整个 ID 列表
	//因为 checkOnce 方法会修改 map，所以不能直接遍历 map 中的 key，否则会导致并发问题
	//同时防止锁一直持有，导致其他 goroutine 无法访问 map
	c.mu.RLock()
	ids := make([]string, 0, len(c.backends))
	for id := range c.backends {
		ids = append(ids, id)
	}
	c.mu.RUnlock()

	for _, backendID := range ids {
		c.checkOnce(backendID)
	}
}

// Start 启动后台 goroutine，定时执行 checkAll 方法
func (c *Checker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		// 先检查一次
		c.checkAll()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.checkAll()
			}
		}
	}()
}

// States 获取所有后端的健康状态 (只返回快照)
func (c *Checker) States() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	states := make(map[string]string)
	for _, backend := range c.backends {
		states[backend.ID] = backend.State.String()
	}
	return states
}

func (c *Checker) Latencies() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	latencies := make(map[string]float64)
	for _, backend := range c.backends {
		latencies[backend.ID] = backend.LastLatency
	}
	return latencies
}
