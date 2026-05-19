# LLM-Router

一个智能 LLM 请求路由网关，支持多后端负载均衡、健康检查、SSE 流式转发和 Prometheus 监控。

## 快速启动

```bash
# 终端 1-3: 3 个 mock backend
go run scripts/mock_backend.go --port=9001 --model=qwen2.5-72b --gpu-type=A100-80G
go run scripts/mock_backend.go --port=9002 --model=qwen2.5-72b --gpu-type=A100-80G
go run scripts/mock_backend.go --port=9003 --model=qwen2.5-7b --gpu-type=A10-24G

# 终端 4: gateway
go run cmd/gateway/main.go
```

## 接口说明

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | OpenAI 兼容的流式对话接口，自动路由到最优后端 |
| `/health` | GET | 网关及后端健康状态概览 |
| `/metrics` | GET | Prometheus 监控指标 |

### `/v1/chat/completions` — 流式对话

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role": "user", "content": "Hello, who are you?"}
    ]
  }'
```

返回 SSE 流式响应，逐 token 转发自最优后端。

### `/health` — 健康检查

```bash
curl http://localhost:8080/health
```

返回示例：
```json
{
  "backends": {
    "vllm-a100-1": "healthy",
    "vllm-a100-2": "healthy",
    "vllm-a10-1": "healthy"
  },
  "status": "ok"
}
```

### `/metrics` — Prometheus 指标

```bash
curl http://localhost:8080/metrics
```

返回 Prometheus 格式的文本指标，包括 QPS、延迟分布、错误计数、在飞请求数等。

## 路由策略

Gateway 采用 Filter → Score → Select 三级决策链：

1. **Filter** — 淘汰不健康或满负载的后端
2. **Score** — 多维度加权打分（延迟权重 40%、队列深度 30%、健康状态 20%、GPU 性能 10%），分数越低越优
3. **Select** — 选中分数最低的后端

## 功能特性

- **智能路由** — 综合后端健康、负载、延迟和 GPU 性能评分，动态选择最优后端
- **SSE 流式转发** — 完整支持 Server-Sent Events，逐行转发 token
- **并发控制** — 基于信号量的 admission control，防止网关过载
- **健康检查** — 定期探测后端健康状态（Healthy / Suspect / Unhealthy）
- **Prometheus 监控** — 内置 QPS、延迟分布（P50/P90/P99）、错误计数、在飞请求数等指标
- **配置驱动** — 通过 YAML 文件声明后端集群信息
