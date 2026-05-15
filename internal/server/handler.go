package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
)

type Server struct {
	config     *config.Config
	httpServer *http.Server

	checker *health.Checker
}

func NewServer(cfg *config.Config, ctx context.Context) *Server {
	mux := http.NewServeMux() //创建私有路由器
	s := &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
			// 绑定到专用路由器
		},
		checker: health.NewChecker(cfg, time.Second*3),
	}
	s.checker.Start(ctx)

	mux.HandleFunc("/health", s.healthHandler) //在专有路由器上注册！！
	return s
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"backends": s.checker.States(),
	})
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
