package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
)

func TestNewChecker(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "backend-1", URL: "http://localhost:8000", Model: "llama", GPUType: "A100"},
			{ID: "backend-2", URL: "http://localhost:8001", Model: "mistral", GPUType: "A100"},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)

	if checker == nil {
		t.Fatal("NewChecker() returned nil")
	}

	states := checker.States()
	if len(states) != 2 {
		t.Errorf("expected 2 backends, got %d", len(states))
	}

	for id, state := range states {
		if state != Suspect.String() {
			t.Errorf("backend %s expected Suspect state, got %s", id, state)
		}
	}
}

func TestStates_Empty(t *testing.T) {
	cfg := &config.Config{}
	checker := NewChecker(cfg, 5*time.Second)

	states := checker.States()
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
}

func TestCheckOnce_HealthyBackend(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: healthyServer.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)

	// Suspect 需要 2 次连续成功才能变为 Healthy
	checker.checkOnce("test-backend")
	checker.checkOnce("test-backend")

	states := checker.States()
	if states["test-backend"] != Healthy.String() {
		t.Errorf("expected Healthy after 2 consecutive successes, got %s", states["test-backend"])
	}
}

func TestCheckOnce_UnhealthyBackend(t *testing.T) {
	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unhealthyServer.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: unhealthyServer.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)

	// Suspect 需要 2 次连续失败才能变为 Unhealthy
	checker.checkOnce("test-backend")
	checker.checkOnce("test-backend")

	states := checker.States()
	if states["test-backend"] != Unhealthy.String() {
		t.Errorf("expected Unhealthy after 2 consecutive failures, got %s", states["test-backend"])
	}
}

func TestCheckOnce_BackendNotFound(t *testing.T) {
	checker := NewChecker(&config.Config{}, 5*time.Second)
	checker.checkOnce("non-existent")
}

func TestCheckOnce_ServerTimeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: slowServer.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)
	checker.client.Timeout = 100 * time.Millisecond

	// Suspect 需要 2 次连续超时失败才能变为 Unhealthy
	checker.checkOnce("test-backend")
	checker.checkOnce("test-backend")

	states := checker.States()
	if states["test-backend"] != Unhealthy.String() {
		t.Errorf("expected Unhealthy after 2 consecutive timeouts, got %s", states["test-backend"])
	}
}

func TestCheckOnce_StateTransitionFullFlow(t *testing.T) {
	var mu sync.Mutex
	failCount := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fc := failCount
		mu.Unlock()
		if fc < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: backend.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)

	// 初始状态为 Suspect
	states := checker.States()
	if states["test-backend"] != Suspect.String() {
		t.Fatalf("expected initial Suspect state, got %v", states["test-backend"])
	}

	checker.checkOnce("test-backend")
	mu.Lock()
	failCount = 1
	mu.Unlock()

	states = checker.States()
	if states["test-backend"] != Suspect.String() {
		t.Fatalf("expected Suspect after 1 fail (need 2), got %v", states["test-backend"])
	}

	checker.checkOnce("test-backend")
	mu.Lock()
	failCount = 2
	mu.Unlock()

	states = checker.States()
	if states["test-backend"] != Unhealthy.String() {
		t.Fatalf("expected Unhealthy after 2 consecutive fails, got %v", states["test-backend"])
	}

	checker.checkOnce("test-backend")
	mu.Lock()
	failCount = 3
	mu.Unlock()

	states = checker.States()
	if states["test-backend"] != Unhealthy.String() {
		t.Fatalf("expected Unhealthy to stay Unhealthy, got %v", states["test-backend"])
	}

	checker.checkOnce("test-backend")
	states = checker.States()
	if states["test-backend"] != Unhealthy.String() {
		t.Fatalf("expected Unhealthy after 1 success (need 3), got %v", states["test-backend"])
	}

	checker.checkOnce("test-backend")
	states = checker.States()
	if states["test-backend"] != Unhealthy.String() {
		t.Fatalf("expected Unhealthy after 2 successes (need 3), got %v", states["test-backend"])
	}

	checker.checkOnce("test-backend")
	states = checker.States()
	if states["test-backend"] != Suspect.String() {
		t.Fatalf("expected Suspect after 3 consecutive successes, got %v", states["test-backend"])
	}
}

func TestStart_ContextCancellation(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: backend.URL},
		},
	}

	checker := NewChecker(cfg, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	checker.Start(ctx)

	time.Sleep(350 * time.Millisecond)

	states := checker.States()
	// 初始检查 + 至少 3 次 tick，足够从 Suspect -> Healthy
	if states["test-backend"] != Healthy.String() {
		t.Errorf("expected Healthy after periodic checks, got %v", states["test-backend"])
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestConcurrentReadWrite(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: backend.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checker.checkOnce("test-backend")
		}()
	}

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checker.States()
		}()
	}

	wg.Wait()
}

func TestCheckAll(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthyServer.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "healthy", URL: healthyServer.URL},
			{ID: "unhealthy", URL: unhealthyServer.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)
	checker.checkAll()

	states := checker.States()

	// 健康后端: Suspect 状态 + 1 次成功 -> 仍为 Suspect
	if states["healthy"] != Suspect.String() {
		t.Errorf("expected Suspect after 1 success from Suspect, got %v", states["healthy"])
	}

	// 不健康后端: Suspect 状态 + 1 次失败 -> 仍为 Suspect
	if states["unhealthy"] != Suspect.String() {
		t.Errorf("expected Suspect after 1 fail from Suspect, got %v", states["unhealthy"])
	}

	checker.checkAll()

	states = checker.States()

	if states["healthy"] != Healthy.String() {
		t.Errorf("expected Healthy after 2 successes, got %v", states["healthy"])
	}

	if states["unhealthy"] != Unhealthy.String() {
		t.Errorf("expected Unhealthy after 2 failures, got %v", states["unhealthy"])
	}
}

func TestLatencyRecorded(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{ID: "test-backend", URL: backend.URL},
		},
	}

	checker := NewChecker(cfg, 5*time.Second)
	checker.checkOnce("test-backend")

	checker.mu.RLock()
	bh := checker.backends["test-backend"]
	checker.mu.RUnlock()

	if bh.LastLatency <= 0 {
		t.Errorf("expected positive latency, got %f", bh.LastLatency)
	}
}
