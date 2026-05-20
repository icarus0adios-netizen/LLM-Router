package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/admission"
	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
	"github.com/icarus0adios-netizen/LLM-Router/internal/proxy"
	"github.com/icarus0adios-netizen/LLM-Router/internal/router"
	"github.com/icarus0adios-netizen/LLM-Router/internal/server"
)

func main() {
	cfg, err := config.Load(configPath())
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	admissionCtrl := admission.NewController(10, 500*time.Millisecond)
	checker := health.NewChecker(cfg, 3*time.Second)
	tracker := router.NewRequestTracker(cfg.Backends)
	scorer := router.NewScorer()
	rtr := router.NewRouter(checker, tracker, scorer, cfg.Backends)
	proxy := proxy.NewProxy(30 * time.Second)

	srv := server.NewServer(cfg, admissionCtrl, checker, proxy, tracker, rtr, scorer)
	checker.Start(context.Background())

	// HTTP server 在后台跑，主线程等信号
	go func() {
		log.Printf("网关启动在端口 %d", cfg.Port)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务异常退出: %v", err)
		}
	}()

	// 等待关闭信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("收到信号 %v，开始优雅关闭", <-quit)

	// 停止 HTTP（等待在飞请求最多 30s）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	// 停止后台任务
	checker.Stop()

	log.Println("网关已安全退出")
}

func configPath() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return "configs/gateway.yaml"
}
