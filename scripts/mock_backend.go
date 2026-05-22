package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// OpenAI-compatible 请求体（只取我们需要的字段）
type ChatRequest struct {
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var (
	//命令行输入参数
	port    int
	model   string
	gpuType string
)

// 初始化命令行参数
func init() {
	flag.IntVar(&port, "port", 9001, "后端显卡端口")
	flag.StringVar(&model, "model", "default-model", "后端模型")
	flag.StringVar(&gpuType, "gpu-type", "unknown", "后端显卡类型")

}

// healthHandler 处理 /health 请求
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fmt.Fprintf(w, "405 Method Not Allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	//healthHandler函数缺少错误处理，应添加JSON编码错误检查
	err := json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"model":    model,
		"gpu_type": gpuType,
	})
	if err != nil {
		log.Println("encode health response error:", err)
	}
}

// chatHandler 处理 /v1/chat/completions 请求
func chatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fmt.Fprintf(w, "Method Not Allowed")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flush, ok := w.(http.Flusher)
	if !ok {
		log.Println("flusher not found")
		return
	}

	var req ChatRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	defer r.Body.Close() // 关闭请求体
	if err != nil {
		log.Println("decode request error:", err)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flush.Flush()
		return
	}

	// 判断数组是否为空
	if len(req.Messages) == 0 {
		log.Println("empty messages")
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flush.Flush()
		return
	}

	msg := req.Messages[len(req.Messages)-1].Content

	//TODO：Split VS Fields  区别！！！！
	tokens := strings.Split(msg, "")

	//构造 标准 OpenAI 格式响应体
	for _, token := range tokens {
		chunk := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"delta": map[string]string{
					"content": token,
				},
				},
			},
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			log.Println("marshal chunk error:", err)
			continue
		}
		//满足 SSE 规范，发送 delta 事件通知客户端当前处理结果
		fmt.Fprintf(w, "data: %s\n\n", data)
		flush.Flush()

		switch gpuType {
		case "A100-80G":
			time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
		case "A100-40G":
			time.Sleep(time.Duration(15+rand.Intn(25)) * time.Millisecond)
		case "A10-24G":
			time.Sleep(time.Duration(80+rand.Intn(70)) * time.Millisecond)
		default:
			time.Sleep(time.Duration(30+rand.Intn(30)) * time.Millisecond)
		}
	}

	//满足 SSE 规范，发送 [DONE] 事件通知客户端处理完成
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flush.Flush()

}

func main() {
	flag.Parse() // 解析命令行参数

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/v1/chat/completions", chatHandler)

	// 启动 mock-backend 的服务
	fmt.Printf("Server is running on port %d, model: %s, gpu_type: %s\n", port, model, gpuType)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}
