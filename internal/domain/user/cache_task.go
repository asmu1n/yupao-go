package user

import (
	"context"
	"log"
	"sync"
	"time"
	"yupao-go/internal/infra/lock"
)

func (s *Service) WarmUpMatchUsers(ctx context.Context) {
	if s.cache == nil {
		return
	}

	// 使用刚刚封装的 RunWithLock，代码变得极其清爽
	err := s.locker.RunWithLock(ctx, lockKey, lockTTL, func() error {
		log.Println("[Cron] 成功抢到分布式锁，开始执行预热...")

		s.warmUpMatchTask(ctx)

		return nil
	})

	// 错误处理：如果没有抢到锁，就静默忽略
	if err == lock.ErrLockFailed {
		log.Println("[Cron] 其他节点正在执行预热，当前节点主动跳过")
		return
	} else if err != nil {
		log.Printf("[Cron] 预热任务发生系统异常: %v", err)
	}
}

// 分批获取用户进行缓存预热
func (s *Service) warmUpMatchTask(ctx context.Context) {

	s.warmUpLock.Lock()
	defer s.warmUpLock.Unlock()

	log.Println("开始预热匹配用户缓存")
	activeSince := time.Now().Add(-matchCacheTTL)

	candidates, err := s.listAllActiveMatchCandidates(ctx, activeSince)
	if err != nil {
		log.Printf("预热匹配用户缓存失败，加载活跃用户失败: %v", err)
		return
	}

	if len(candidates) == 0 {
		log.Println("预热匹配用户缓存结束: 无活跃用户")
		return
	}

	jobs := make(chan *User, len(candidates))
	var wg sync.WaitGroup

	for i := 0; i < warmWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				if ctx.Err() != nil {
					return
				}
				s.warmUpSingleUser(ctx, u, candidates)
			}
		}()
	}

	for _, u := range candidates {
		if ctx.Err() != nil {
			break
		}
		jobs <- u
	}
	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		log.Printf("预热匹配用户缓存结束（提前取消）: %v", err)
		return
	}
	log.Printf("预热匹配用户缓存结束: 用户数=%d", len(candidates))

}

func (s *Service) warmUpSingleUser(ctx context.Context, loginUser *User, candidates []*User) {
	if len(loginUser.ParseTags()) == 0 {
		return
	}

	nums := []int{10, 20}
	for _, num := range nums {
		key := matchCacheKey(loginUser.ID, num)
		var dst []*User
		err := s.cache.Once(ctx, key, matchCacheTTL, &dst, func() (any, error) {
			return s.matchUsersFromCandidates(ctx, num, loginUser, candidates)
		})
		if err != nil {
			log.Printf("预热匹配缓存失败 key=%s: %v", key, err)
		}
	}

}

// 无效化匹配缓存
func (s *Service) invalidateMatchCache(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}
	keys := make([]string, 0, maxMatchNum)
	for n := 1; n <= maxMatchNum; n++ {
		keys = append(keys, matchCacheKey(userID, n))
	}
	_ = s.cache.Delete(ctx, keys...)
}
