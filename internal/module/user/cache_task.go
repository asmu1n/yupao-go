package user

import (
	"context"
	"sync"
	"time"

	"yupao-go/internal/pkg/logger"
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

var warmupLog = logger.Module("user").With(logger.FieldPurpose, logger.PurposeJob)

// WarmUpMatchUsers 定时预热匹配缓存的入口。
// 通过分布式锁保证多实例下仅一个节点执行；抢锁失败则静默跳过。
func (s *Service) WarmUpMatchUsers(ctx context.Context) {
	if s.cache == nil || s.locker == nil {
		return
	}

	err := s.locker.RunWithLock(ctx, lockKey, lockTTL, func() error {
		warmupLog.Info("warmup acquired lock",
			logger.FieldEvent, "warmup.lock_acquired",
		)
		s.warmUpMatchTask(ctx)
		return nil
	})

	if err == port.ErrLockFailed {
		warmupLog.Info("warmup skipped, lock held by another instance",
			logger.FieldEvent, "warmup.skip_lock",
		)
		return
	}
	if err != nil {
		warmupLog.Error("warmup system error",
			logger.FieldEvent, "warmup.error",
			logger.FieldErr, err,
			logger.FieldPurpose, logger.PurposeAlert,
		)
	}
}

// warmUpMatchTask 加载活跃候选池，并对其中用户并发预热匹配缓存。
func (s *Service) warmUpMatchTask(ctx context.Context) {
	s.warmUpLock.Lock()
	defer s.warmUpLock.Unlock()

	start := time.Now()
	warmupLog.Info("warmup started",
		logger.FieldEvent, "warmup.start",
	)

	candidates, err := s.loadActiveCandidates(ctx)
	if err != nil {
		warmupLog.Error("warmup load candidates failed",
			logger.FieldEvent, "warmup.load_error",
			logger.FieldErr, err,
		)
		return
	}

	if len(candidates) == 0 {
		warmupLog.Info("warmup finished, no active users",
			logger.FieldEvent, "warmup.done",
			"candidates", 0,
			"duration_ms", time.Since(start).Milliseconds(),
		)
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
		warmupLog.Warn("warmup cancelled",
			logger.FieldEvent, "warmup.cancelled",
			logger.FieldErr, err,
			"candidates", len(candidates),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}
	warmupLog.Info("warmup finished",
		logger.FieldEvent, "warmup.done",
		"candidates", len(candidates),
		"duration_ms", time.Since(start).Milliseconds(),
	)
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
		err := s.cache.Once(ctx, key, matchCacheTTL, &dst, func() (any, error) {
			return s.rankMatches(ctx, num, loginUser, candidates)
		})
		if err != nil {
			warmupLog.Error("warmup single key failed",
				logger.FieldEvent, "warmup.key_error",
				logger.FieldErr, err,
				"key", key,
				"user_id", loginUser.ID,
				"num", num,
			)
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
	if err := s.cache.Delete(ctx, keys...); err != nil {
		logger.Module("user").Warn("invalidate match cache failed",
			logger.FieldPurpose, logger.PurposeCache,
			logger.FieldEvent, "cache.match.invalidate_error",
			logger.FieldErr, err,
			"user_id", userID,
		)
	}
}
