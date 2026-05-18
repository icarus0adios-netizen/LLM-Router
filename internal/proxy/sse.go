package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Proxy struct {
	client *http.Client
}

func NewProxy(internal time.Duration) *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: internal,
		},
	}
}

func (p *Proxy) Forward(ctx context.Context, w http.ResponseWriter, backendURL string, body io.Reader) error {
	//1-构建后端请求
	targetURL := backendURL + "/v1/chat/completions"

	backendCtx, cancel := context.WithTimeout(ctx, p.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(backendCtx, http.MethodPost, targetURL, body)
	if err != nil {
		return fmt.Errorf("create backend request failed: %v", err)
	}

	//2-发送后端请求
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send backend request failed: %v", err)
	}
	defer resp.Body.Close()

	//3-检查后端响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend response status code: %d", resp.StatusCode)
	}

	//4-设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-cache")

	//5-获取flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer is not a flusher")
	}

	//6-逐行转发
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		//检查是否中断
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Text()
		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return fmt.Errorf("forward response line failed: %v", err)
		}
		flusher.Flush()
	}
	if scanner.Err() != nil {
		return fmt.Errorf("scan response body failed: %v", scanner.Err())
	}
	return nil
}
