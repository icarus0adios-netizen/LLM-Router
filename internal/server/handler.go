package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
)

type Server struct {
	config     *config.Config
	httpServer *http.Server
}

func NewServer(cfg *config.Config) *Server {
	mux := http.NewServeMux() //创建私有路由器
	s := &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
			// 绑定到专用路由器
		},
	}
	mux.HandleFunc("/health", s.healthHandler) //在专有路由器上注册！！
	return s
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
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
