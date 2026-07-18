package scheduler

import (
	"context"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	client *cron.Cron
	// 主要作用是保证调度器的启动和停止是线程安全的
	mu      sync.Mutex
	started bool
}

func New() *Scheduler {
	logger := cron.VerbosePrintfLogger(log.Default())

	return &Scheduler{
		client: cron.New(
			cron.WithSeconds(),
			cron.WithLogger(logger),
			cron.WithChain(
				cron.Recover(logger),
				cron.SkipIfStillRunning(logger),
			),
		),
	}
}

// Schedule 注册任务，不会启动调度器。
func (s *Scheduler) Schedule(spec string, cmd func()) (cron.EntryID, error) {
	id, err := s.client.AddFunc(spec, cmd)
	if err != nil {
		return 0, err
	}
	log.Printf("定时任务已注册:%v \n", id)
	return id, nil
}

// Start 启动调度器，多次调用幂等。
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}
	s.client.Start()
	s.started = true
}

// Stop 停止调度器并等待正在执行的任务结束。
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	// 获取停止任务的进度并更新运行标记以及解除互斥锁
	stopCtx := s.client.Stop()
	s.started = false
	s.mu.Unlock()

	if ctx == nil {
		<-stopCtx.Done()
		return nil
	}

	// select 多路判断，保留外部上下文控制来决定阻塞时长
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
