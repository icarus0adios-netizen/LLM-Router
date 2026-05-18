package admission

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	ctrl := NewController(2, 100*time.Millisecond)

	// 获取 2 个，应该都成功
	if err := ctrl.Acquire(context.Background()); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if err := ctrl.Acquire(context.Background()); err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}

	// 第 3 个应该超时
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ctrl.Acquire(ctx); err == nil {
		t.Fatal("third acquire should have timed out")
	}

	// 释放一个，第 3 个应该能拿到了
	ctrl.Release()
	if err := ctrl.Acquire(context.Background()); err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	ctrl := NewController(1, 10*time.Second)

	// 占满
	if err := ctrl.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 用已取消的 ctx 尝试获取
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ctrl.Acquire(ctx); err == nil {
		t.Fatal("should have failed with canceled context")
	}
}

func TestConcurrentSafety(t *testing.T) {
	ctrl := NewController(100, time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ctrl.Acquire(context.Background()); err != nil {
				t.Errorf("concurrent acquire failed: %v", err)
				return
			}
			time.Sleep(time.Millisecond)
			ctrl.Release()
		}()
	}
	wg.Wait()
}
