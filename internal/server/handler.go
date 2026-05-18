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
	"github.com/icarus0adios-netizen/LLM-Router/internal/proxy"
)

type Server struct {
	config     *config.Config
	httpServer *http.Server

	checker       *health.Checker
	proxy         *proxy.Proxy
	admissionCtrl *admission.Controller
}

func NewServer(cfg *config.Config, ctx context.Context, admissionCtrl *admission.Controller) *Server {
	mux := http.NewServeMux() //创建私有路由器
	s := &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
			// 绑定到专用路由器
		},
		checker:       health.NewChecker(cfg, time.Second*3),
		proxy:         proxy.NewProxy(30 * time.Second),
		admissionCtrl: admissionCtrl,
	}
	s.checker.Start(ctx)

	mux.HandleFunc("/health", s.healthHandler) //在专有路由器上注册！！
	mux.HandleFunc("/v1/chat/completions", s.chatHandler)
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

	// 申请信号量
	if err := s.admissionCtrl.Acquire(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	defer s.admissionCtrl.Release()

	// Week 3 之前：硬编码选第一个 healthy 后端
	// 今天：从 checker 拿健康后端列表，选第一个

	//TODO : 下周用Router替换
	backendURL := "http://localhost:9001"
	if err := s.proxy.Forward(r.Context(), w, backendURL, r.Body); err != nil {
		log.Fatalf("代理转发失败：%v", err)
	}
}

// Start 启动server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// 使用 conetxt 优雅关闭server
// TODO  ： Week 4 才会用到，暂时只写一个骨架
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
