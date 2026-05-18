package main

import (
	"context"
	"log"
	"time"

	"github.com/icarus0adios-netizen/LLM-Router/internal/admission"
	"github.com/icarus0adios-netizen/LLM-Router/internal/config"
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
	srv := server.NewServer(cfg, ctx, admissionCtrl)

	if err := srv.Start(); err != nil {
		log.Fatalf("启动server失败: %v\n", err)
		return
	}
}
