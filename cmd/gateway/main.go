package main

import (
	"log"

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
	srv := server.NewServer(cfg)

	if err := srv.Start(); err != nil {
		log.Fatalf("启动server失败: %v\n", err)
		return
	}
}
