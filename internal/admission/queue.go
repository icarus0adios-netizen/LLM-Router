package admission

import (
	"context"
	"fmt"
	"time"
)

type Controller struct {
	sem     chan struct{}
	timeout time.Duration
}

func NewController(maxConcurrency int, timeout time.Duration) *Controller {
	return &Controller{
		sem:     make(chan struct{}, maxConcurrency),
		timeout: timeout,
	}
}

func (c *Controller) Acquire(ctx context.Context) error {
	// 先尝试一次，获取信号量
	select {
	case c.sem <- struct{}{}:
		return nil
	default:
	}

	// 等待超时
	//99% 以上的请求都是走的上面“尝试的路径”，所以将Newtikcer分割开来，
	//避免： 即使 sem 有空位也要分配一次 time.After() 的 Timer ！！！！
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("Too Many Requests")
	case c.sem <- struct{}{}:
		return nil
	}
}

func (c *Controller) Release() {
	<-c.sem
}
