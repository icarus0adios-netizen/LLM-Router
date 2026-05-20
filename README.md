# LLM Router

一个高性能 LLM 推理网关，实现**多维度加权路由**、**SSE 流式代理**、**三态健康状态机**、**并发准入控制**和 **Prometheus 可观测性**。

---

## 架构

```
                        POST /v1/chat/completions
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         GATEWAY                                  │
│                                                                   │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐         │
│  │  Admission    │──▶│   Router     │──▶│  SSE Proxy   │         │
│  │  (并发控制)    │   │  Filter→Score│   │  (流式转发)   │         │
│  │              │   │  →Select    │   │              │         │
│  └──────────────┘   └──────┬───────┘   └──────────────┘         │
│                            │                                      │
│                     ┌──────▼───────┐                             │
│                     │ Health Checker│                             │
│                     │ (三态状态机)   │                             │
│                     └──────────────┘                             │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Prometheus 指标 (/metrics)                    │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
   ┌──────────┐        ┌──────────┐        ┌──────────┐
   │ vLLM #1  │        │ vLLM #2  │        │ vLLM #3  │
   │ A100-80G │        │ A100-80G │        │ A10-24G  │
   └──────────┘        └──────────┘        └──────────┘
```

---

## 核心功能

| 功能 | 说明 |
|------|------|
| **Filter→Score→Select 路由** | 四维度加权打分：延迟 (40%)、排队深度 (30%)、健康状态 (20%)、GPU 类型 (10%) |
| **三态健康状态机** | Healthy → Suspect → Unhealthy，可配置阈值，防止抖动 |
| **并发准入控制** | 基于 Channel Semaphore，超限返回 429，等待超时 500ms |
| **SSE 流式代理** | 实时逐 token 转发，`bufio.Scanner` + `http.Flusher`，支持客户端断开检测 |
| **Prometheus 可观测性** | 6 项指标：请求计数、延迟直方图、错误计数、健康状态、在飞请求数、路由打分 |
| **优雅关闭** | SIGINT/SIGTERM → 排空在飞请求（30s deadline）→ 关闭后台 goroutine → 安全退出 |

---

## 性能测试

测试环境：3 个异构 Mock Backend（A100-80G ×2 + A10-24G ×1），mock_backend token 间隔 50-100ms。

| 场景 | 平均延迟 | P99 延迟 | QPS | 429 率 |
|------|:------:|:------:|:---:|:-----:|
| 直连 Backend (c=1) | 378ms | 445ms | 2.65 | - |
| 经网关 (c=5) | 1065ms | 1183ms | 4.64 | 0% |
| 经网关 (c=10, 30s 压测) | 680ms | 783ms | **14.57** | 0% |
| 经网关 (c=20, 超并发) | 816ms | - | 23.61 | **50%** |

关键结论：

- **代理层开销 < 1ms**：`resp wait` 直连 0.3ms vs 网关 0.5ms，转发本身几乎零开销
- **准入控制生效**：c=20 时 50% 请求返回 429，成功保护后端不过载
- **稳定吞吐**：c=10 下 30 秒处理 447 请求，零错误

---

## 快速开始

### 1. 启动 Mock Backend（3 个终端）

```bash
go run scripts/mock_backend.go --port=9001 --model=qwen2.5-72b --gpu-type=A100-80G
go run scripts/mock_backend.go --port=9002 --model=qwen2.5-72b --gpu-type=A100-80G
go run scripts/mock_backend.go --port=9003 --model=qwen2.5-7b  --gpu-type=A10-24G
```

### 2. 启动网关

```bash
go run cmd/gateway/main.go
# 输出: 网关启动在端口 8080
```

### 3. 发送流式请求

```bash
curl -N -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"你好，世界"}]}'
```

### 4. 查看健康状态

```bash
curl http://localhost:8080/health | jq
```

### 5. 查看 Prometheus 指标

```bash
curl http://localhost:8080/metrics | grep gateway_
```

---

## 设计决策

| 决策 | 选择 | 拒绝的方案 |
|------|------|-----------|
| 并发控制 | Channel Semaphore（buffered channel） | Token Bucket：LLM 瓶颈在并发数（KV Cache），不在速率 |
| 健康检查 | 三态 FSM（Healthy/Suspect/Unhealthy） | 两态：一次网络抖动就摘除，太激进 |
| 路由模型 | 多维度加权打分 | Round-Robin：无法感知异构 GPU 和实时负载 |
| 负载追踪 | RequestTracker（Inc/Dec 计数） | 中心化队列：单点故障 + 额外延迟 |
| 可观测性 | Prometheus client_golang | OpenTelemetry：更重，对于推理网关接入成本高 |
| HTTP 框架 | 标准库 `net/http` | Gin/Fiber：引入不必要的抽象，标准库面试更受认可 |
| 配置加载 | YAML + 环境变量覆盖 | 硬编码 / 命令行参数：不满足 12-Factor App |

---


## 路由算法详解

```
Route()
  │
  ├─ Filter（硬约束）
  │   ├─ 淘汰 Unhealthy 后端
  │   ├─ 淘汰满负载后端（Active ≥ Max）
  │   └─ 淘汰模型不匹配后端
  │
  ├─ Score（四维度加权，0-100 分，越低越好）
  │   ├─ Latency  (40%): 最近 /health 延迟 ×100
  │   ├─ QueueDepth (30%): (在飞请求/最大并发) ×100
  │   ├─ Health   (20%): Healthy=0, Suspect=50
  │   └─ GPU      (10%): A100=0, A10=20
  │
  └─ Select: 选最低分
```

---

## 健康状态机

```
                  连续失败 ≥ 3 次
     Healthy ──────────────────────▶ Suspect
        ▲                             │
        │       连续成功 ≥ 2 次         │ 连续失败 ≥ 2 次
        │                             │
        └─────────────────────────────┘        │
                 恢复试探                      ▼
                              Unhealthy ──────▶ Suspect
                              (摘除流量)   连续成功 ≥ 3 次
```

---

## Prometheus 指标

| 指标 | 类型 | 说明 |
|------|------|------|
| `gateway_requests_total` | Counter | 请求总数，按后端和状态分组 |
| `gateway_request_duration_seconds` | Histogram | 请求耗时分布（bucket: 0.05~30s） |
| `gateway_errors_total` | Counter | 错误计数，按原因分组 |
| `gateway_backend_health` | Gauge | 后端健康状态（0=Healthy, 1=Suspect, 2=Unhealthy） |
| `gateway_backend_active_requests` | Gauge | 后端当前在飞请求数 |
| `gateway_router_score` | Histogram | 路由选中后端的分数分布 |

---

## 运行测试

```bash
# 全部测试（含数据竞争检测）
go test -race -timeout 60s ./...

# 仅集成测试
go test -v -race -run TestIntegration ./internal/server/

# 格式化 + 静态分析
go fmt ./...
go vet ./...
```

---
