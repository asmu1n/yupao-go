package user

import (
	"context"
	"fmt"
	"time"

	"yupao-go/internal/port"
)

const (
	// matchCacheTTL 匹配结果缓存过期时间。
	matchCacheTTL = 60 * time.Minute
	// matchActiveWindow 在线匹配与预热共用的活跃候选窗口。
	matchActiveWindow = 7 * 24 * time.Hour
	// maxMatchNum 匹配接口允许的最大推荐数量。
	maxMatchNum = 20
	// warmBatchSize 游标分批加载活跃用户时的每批大小。
	warmBatchSize = 200
	// warmWorkers 预热任务并发 worker 数。
	warmWorkers = 4
)

// MatchUsers 推荐与 loginUser 标签最相似的 num 个活跃用户。
//
// 调用链（尽量短）：
//
//	MatchUsers
//	  └─ cache hit  → 直接返回
//	  └─ cache miss → match
//	                    ├─ loadActiveCandidates   // 拉活跃池（与预热共用）
//	                    └─ rankMatches            // 编辑距离 + Top-K + 回表
func (s *Service) MatchUsers(ctx context.Context, loginUser *User, num int) ([]*User, error) {
	if len(loginUser.ParseTags()) == 0 {
		return nil, nil
	}

	compute := func() ([]*User, error) {
		fmt.Println("未命中缓存")
		return s.match(ctx, loginUser, num)
	}

	if s.cache == nil {
		return s.match(ctx, loginUser, num)
	}
	return port.TryFetch(ctx, s.cache, matchCacheKey(loginUser.ID, num), matchCacheTTL, compute)
}

// match 在线 miss / 无缓存时的完整计算：加载候选池 → 排序取 Top-K。
func (s *Service) match(ctx context.Context, loginUser *User, num int) ([]*User, error) {
	// 只获取活跃用户
	candidates, err := s.loadActiveCandidates(ctx)
	if err != nil {
		return nil, err
	}
	return s.rankMatches(ctx, num, loginUser, candidates)
}

// loadActiveCandidates 加载匹配候选池（在线与预热唯一入口，保证缓存语义一致）。
// 条件：近 matchActiveWindow 有更新、状态正常、未删除、有 tags；按 ID 游标分批。
func (s *Service) loadActiveCandidates(ctx context.Context) ([]*User, error) {
	activeSince := time.Now().Add(-matchActiveWindow)
	afterID := int64(0)
	all := make([]*User, 0, warmBatchSize)

	// 按照批次完成活跃用户查询
	for {
		// 根据当前游标查询活跃用户
		batch, err := s.repo.ListActiveMatchCandidates(ctx, afterID, warmBatchSize, activeSince)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		// 插入当前批次的查询结构
		all = append(all, batch...)
		// 更新查询游标
		afterID = batch[len(batch)-1].ID
		// 根据当前数据量判断是否查询完毕
		if len(batch) < warmBatchSize {
			break
		}
	}
	return all, nil
}

// rankMatches 在已给定的候选池上按标签编辑距离选出最近的 num 个用户。
func (s *Service) rankMatches(ctx context.Context, num int, loginUser *User, candidates []*User) ([]*User, error) {
	// 解析当前用户的 tag
	loginTags := loginUser.ParseTags()

	scored := make([]scoredUser, 0, len(candidates))
	// 计算活跃用户中与当前用户的匹配度
	for _, u := range candidates {
		// 过滤当前用户
		if u.ID == loginUser.ID {
			continue
		}
		uTags := u.ParseTags()
		if len(uTags) == 0 {
			continue
		}
		scored = append(scored, scoredUser{
			userID:   u.ID,
			distance: minDistance(loginTags, uTags),
		})
	}
	// 排序并过滤
	ids := topKNearest(scored, num)
	if len(ids) == 0 {
		return nil, nil
	}

	// 获取完整用户信息
	matched, err := s.repo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	// 维护一个字典表
	byID := make(map[int64]*User, len(matched))
	for _, u := range matched {
		byID[u.ID] = u
	}

	// 借助字典表按照 排序后的 ids 录入数据
	result := make([]*User, 0, len(ids))
	for _, id := range ids {
		if u, ok := byID[id]; ok {
			result = append(result, u)
		}
	}
	return result, nil
}

// matchCacheKey 生成匹配结果缓存 key：yupao:match:{userID}:{num}。
func matchCacheKey(userID int64, num int) string {
	return fmt.Sprintf("yupao:match:%d:%d", userID, num)
}
