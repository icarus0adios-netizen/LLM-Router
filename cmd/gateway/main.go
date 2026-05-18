package main

import (
	"context"
	"log"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/admission"
	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
	"github.com/icarus0adios-netizen/LLM-Router/internal/health"
	"github.com/icarus0adios-netizen/LLM-Router/internal/proxy"
	"github.com/icarus0adios-netizen/LLM-Router/internal/router"
	"github.com/icarus0adios-netizen/LLM-Router/internal/server"
)

func main() {
	cfgPath := "configs/gateway.yaml"
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v\n", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	admissionCtrl := admission.NewController(10, time.Millisecond*10)

	tracker := router.NewRequestTracker(cfg.Backends)
	checker := health.NewChecker(cfg, time.Second*3)
	proxy := proxy.NewProxy(30 * time.Second)
	scorer := router.NewScorer()
	rtr := router.NewRouter(checker, tracker, scorer, cfg.Backends)

	srv := server.NewServer(cfg, ctx, admissionCtrl, checker, proxy, tracker, rtr, scorer)

	if err := srv.Start(); err != nil {
		log.Fatalf("启动server失败: %v\n", err)
		return
	}
}
