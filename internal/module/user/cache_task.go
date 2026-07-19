package user

import (
	"context"
	"log"
	"sync"
	"time"

	"yupao-go/internal/port"
)

const (
	// lockTTL 预热分布式锁过期时间，应覆盖单次任务最坏耗时。
	lockTTL = 10 * time.Minute
	// lockKey 预热任务在 Redis 中的分布式锁 key。
	lockKey = "lock:cron:warmup_match_users"
)

// warmUpNums 预热时优先填充的推荐数量；与接口常见取值对齐，其余 num 走在线懒加载。
var warmUpNums = []int{10, 20}

// WarmUpMatchUsers 定时预热匹配缓存的入口。
// 通过分布式锁保证多实例下仅一个节点执行；抢锁失败则静默跳过。
func (s *Service) WarmUpMatchUsers(ctx context.Context) {
	if s.cache == nil || s.locker == nil {
		return
	}

	// 尝试从 redis 获取分布式锁
	err := s.locker.RunWithLock(ctx, lockKey, lockTTL, func() error {
		log.Println("[Cron] 成功抢到分布式锁，开始执行预热...")
		s.warmUpMatchTask(ctx)
		return nil
	})

	// 错误处理：如果没有抢到锁，就静默忽略
	if err == port.ErrLockFailed {
		log.Println("[Cron] 其他节点正在执行预热，当前节点主动跳过")
		return
	} else if err != nil {
		log.Printf("[Cron] 预热任务发生系统异常: %v", err)
	}
}

// warmUpMatchTask 加载活跃候选池，并对其中用户并发预热匹配缓存。
// 候选池与在线 MatchUsers 相同，保证写入 key 的语义一致。
func (s *Service) warmUpMatchTask(ctx context.Context) {
	s.warmUpLock.Lock()
	defer s.warmUpLock.Unlock()

	log.Println("开始预热匹配用户缓存")

	// 与 MatchUsers miss 路径共用 loadMatchCandidates，避免候选集分叉。
	candidates, err := s.loadMatchCandidates(ctx)
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

// warmUpSingleUser 为单个用户补齐常见 num 的匹配缓存（仅填冷 key，不强制刷新）。
func (s *Service) warmUpSingleUser(ctx context.Context, loginUser *User, candidates []*User) {
	if len(loginUser.ParseTags()) == 0 {
		return
	}

	for _, num := range warmUpNums {
		if ctx.Err() != nil {
			return
		}
		key := matchCacheKey(loginUser.ID, num)
		var dst []*User
		// Once：仅补冷 key；计算结果与在线 matchUsers 同源（同一 candidates）。
		err := s.cache.Once(ctx, key, matchCacheTTL, &dst, func() (any, error) {
			return s.matchUsersFromCandidates(ctx, num, loginUser, candidates)
		})
		if err != nil {
			log.Printf("预热匹配缓存失败 key=%s: %v", key, err)
		}
	}
}

// invalidateMatchCache 用户资料变更后删除其全部匹配缓存（num = 1..maxMatchNum）。
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
