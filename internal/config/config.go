package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config 配置结构体，包含端口号和后端配置
// yaml 配置示例
/*
port: 8080
backends:
  - id: "backend-1"
    url: "http://localhost:8000"
    model: "llama-3.1-8b"
    gpu_type: "A100"
  - id: "backend-2"
    url: "http://localhost:8001"
    model: "mistral-7b"
    gpu_type: "A100"
*/

type Config struct {
	Port     int             `yaml:"port"`
	Backends []BackendConfig `yaml:"backends"`
}

type BackendConfig struct {
	ID      string `yaml:"id"`
	URL     string `yaml:"url"`
	Model   string `yaml:"model"`
	GPUType string `yaml:"gpu_type"`
}

// overrideFromEnv 检查环境变量中的 port，若存在则覆盖 YAML 中的配置值
func overrideFromEnv(cfg *Config) {
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
}

// Load 读取指定路径的 YAML 配置文件，解析为 Config 结构体后返回
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 YAML 配置失败: %w", err)
	}
	overrideFromEnv(&cfg)
	return &cfg, nil
}
