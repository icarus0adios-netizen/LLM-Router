package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/admission"
	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
	"github.com/icarus0adios-netizen/LLM-Router/internal/metrics"
	"github.com/icarus0adios-netizen/LLM-Router/internal/proxy"
	"github.com/icarus0adios-netizen/LLM-Router/internal/router"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	config     *config.Config
	httpServer *http.Server

	checker       *health.Checker
	proxy         *proxy.Proxy
	admissionCtrl *admission.Controller
	tracker       *router.RequestTracker
	router        *router.Router
	scorer        *router.Scorer
}

func NewServer(cfg *config.Config,
	admissionCtrl *admission.Controller,
	checker *health.Checker,
	proxy *proxy.Proxy,
	tracker *router.RequestTracker,
	router *router.Router,
	scorer *router.Scorer,
) *Server {
	mux := http.NewServeMux() //创建私有路由器
	s := &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
			// 绑定到专用路由器
		},
		checker:       checker,
		proxy:         proxy,
		admissionCtrl: admissionCtrl,
		tracker:       tracker,
		router:        router,
		scorer:        scorer,
	}

	mux.HandleFunc("/health", s.healthHandler) //在专有路由器上注册！！
	mux.HandleFunc("/v1/chat/completions", s.chatHandler)
	mux.Handle("/metrics", promhttp.Handler())
	return s
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"backends": s.checker.States(),
	})
}

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 记录请求开始时间
	start := time.Now()

	// 申请信号量
	if err := s.admissionCtrl.Acquire(r.Context()); err != nil {
		metrics.RecordError("admission_rejected")
		metrics.RecordRequest("", http.StatusTooManyRequests, time.Since(start).Seconds())
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	defer s.admissionCtrl.Release()

	// Route() 内部完成 Filter→Score→Select + tracker.Inc(winnerID)，
	// 保证并发请求的 Snapshot 能看到前一个请求已占用的负载。
	// handler 只需负责 Dec，在请求完成时释放。
	id, backendURL, err := s.router.Route()
	if err != nil {
		metrics.RecordRequest("", http.StatusServiceUnavailable, time.Since(start).Seconds())
		metrics.RecordError("no_backend")
		log.Printf("路由失败: %v", err)
		http.Error(w, `{"error":"no available backend"}`, http.StatusServiceUnavailable)
		return
	}
	defer s.tracker.Dec(id) //！！！！！

	// 更新 Prometheus gauge
	load := s.tracker.Snapshot()
	for backendID, count := range load {
		metrics.SetBackendActiveRequests(backendID, float64(count.Active))
	}

	log.Printf("[route] 选中后端: %s (%s)", id, backendURL)

	if err := s.proxy.Forward(r.Context(), w, backendURL, r.Body); err != nil {
		metrics.RecordError("backend_error")
		metrics.RecordRequest(id, http.StatusBadGateway, time.Since(start).Seconds())
		log.Printf("代理转发失败 (backend=%s): %v", backendURL, err)
		return
	}

	metrics.RecordRequest(id, http.StatusOK, time.Since(start).Seconds())
}

// Start 启动server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// 使用 conetxt 优雅关闭server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
