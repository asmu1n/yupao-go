package user

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"yupao-go/internal/port"
	"yupao-go/internal/pkg/response"
)

var validAccountPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

const (
	// matchCacheTTL 匹配结果缓存过期时间。
	matchCacheTTL = 60 * time.Minute
	// matchActiveWindow 在线匹配与预热共用的活跃候选窗口。
	// 仅在该时间范围内有更新、状态正常且带 tags 的用户会进入匹配池。
	matchActiveWindow = 7 * 24 * time.Hour
	// maxMatchNum 匹配接口允许的最大推荐数量。
	maxMatchNum = 20
	// warmBatchSize 游标分批加载活跃用户时的每批大小。
	warmBatchSize = 200
	// warmWorkers 预热任务并发 worker 数。
	warmWorkers = 4
)

type Service struct {
	repo  Repository
	cache port.Cache
	// warmUpLock 进程内预热互斥，防止同进程重复执行预热。
	warmUpLock sync.Mutex
	// locker 分布式锁，多实例下保证预热任务互斥。
	locker port.Locker
}

// NewService 构造用户领域服务。
func NewService(repo Repository, c port.Cache, locker port.Locker) *Service {
	return &Service{repo: repo, cache: c, locker: locker}
}

// matchCacheKey 生成匹配结果缓存 key：yupao:match:{userID}:{num}。
func matchCacheKey(userID int64, num int) string {
	return fmt.Sprintf("yupao:match:%d:%d", userID, num)
}

// Register 用户注册，校验参数 + 查重 + bcrypt 加密后入库，返回新用户 ID
func (s *Service) Register(ctx context.Context, p RegisterParams) (int64, error) {
	if !validAccountPattern.MatchString(p.UserAccount) {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "账号包含特殊字符")
	}

	exists, err := s.repo.ExistsByAccount(ctx, p.UserAccount)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "账号重复")
	}

	exists, err = s.repo.ExistsByPlanetCode(ctx, p.PlanetCode)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "编号重复")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(p.UserPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, response.NewBizErrorWithDetail(response.SystemError, "密码加密失败")
	}

	u := &User{
		UserAccount: p.UserAccount,
		Password:    string(hashed),
		PlanetCode:  p.PlanetCode,
	}
	return s.repo.Create(ctx, u)
}

// Login 用户登录，校验账号密码后返回脱敏用户信息
func (s *Service) Login(ctx context.Context, account, password string) (*User, error) {
	if !validAccountPattern.MatchString(account) {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号包含特殊字符")
	}

	u, err := s.repo.GetByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号或密码错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, response.NewBizErrorWithDetail(response.ParamsError, "账号或密码错误")
	}

	return u, nil
}

// GetByID 根据 ID 获取脱敏用户信息
func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	return u, nil
}

// Update 更新用户信息，管理员可改任意用户，普通用户仅可改自己
func (s *Service) Update(ctx context.Context, targetID int64, u *User, callerID int64) error {
	caller, err := s.repo.GetByID(ctx, callerID)
	if err != nil {
		return err
	}
	if caller == nil {
		return response.NewBizError(response.NotFound)
	}
	isAdmin := s.isAdmin(caller)

	if targetID <= 0 {
		return response.NewBizError(response.ParamsError)
	}
	if !isAdmin && targetID != callerID {
		return response.NewBizError(response.NoAuth)
	}

	old, err := s.repo.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if old == nil {
		return response.NewBizError(response.NotFound)
	}

	if err := s.repo.Update(ctx, targetID, u); err != nil {
		return err
	}

	s.invalidateMatchCache(ctx, targetID)
	return nil
}

// SearchByTags 根据标签列表搜索用户，内存过滤匹配所有标签的用户
func (s *Service) SearchByTags(ctx context.Context, tagNames []string) ([]*User, error) {
	if len(tagNames) == 0 {
		return nil, response.NewBizError(response.ParamsError)
	}

	users, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	tagSet := make(map[string]struct{}, len(tagNames))
	for _, t := range tagNames {
		tagSet[t] = struct{}{}
	}

	var result []*User
	for _, u := range users {
		userTags := u.ParseTags()
		if len(userTags) == 0 {
			continue
		}
		if containsAll(userTags, tagSet) {
			result = append(result, u)
		}
	}
	return result, nil
}

// MatchUsers 基于标签编辑距离，在活跃候选池中推荐最相似的 num 个用户。
// 优先读缓存；miss 时走 matchUsers 计算并回填。候选池与预热一致，见 loadMatchCandidates。
func (s *Service) MatchUsers(ctx context.Context, num int, loginUser *User) ([]*User, error) {
	if len(loginUser.ParseTags()) == 0 {
		return nil, nil
	}

	if s.cache == nil {
		return s.matchUsers(ctx, num, loginUser)
	}

	return port.TryFetch(ctx, s.cache, matchCacheKey(loginUser.ID, num), matchCacheTTL, func() ([]*User, error) {
		return s.matchUsers(ctx, num, loginUser)
	})
}

// loadMatchCandidates 加载匹配候选池（在线与预热共用，保证缓存语义一致）。
func (s *Service) loadMatchCandidates(ctx context.Context) ([]*User, error) {
	activeSince := time.Now().Add(-matchActiveWindow)
	return s.listAllActiveMatchCandidates(ctx, activeSince)
}

// listAllActiveMatchCandidates 按 ID 游标分批拉取活跃匹配候选，避免一次全表加载。
func (s *Service) listAllActiveMatchCandidates(ctx context.Context, activeSince time.Time) ([]*User, error) {
	afterID := int64(0)
	all := make([]*User, 0, warmBatchSize)

	for {
		batch, err := s.repo.ListActiveMatchCandidates(ctx, afterID, warmBatchSize, activeSince)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)
		// 记录当前批次数据的最后 Id
		afterID = batch[len(batch)-1].ID

		// 如果批次获取的数据量小于限制说明后面没有数据，直接取消并返回数据
		if len(batch) < warmBatchSize {
			break
		}
	}

	return all, nil
}

// matchUsers 缓存 miss 时的计算入口：加载候选池后执行匹配算法。
func (s *Service) matchUsers(ctx context.Context, num int, loginUser *User) ([]*User, error) {
	candidates, err := s.loadMatchCandidates(ctx)
	if err != nil {
		return nil, err
	}
	return s.matchUsersFromCandidates(ctx, num, loginUser, candidates)
}

// matchUsersFromCandidates 在给定候选池内按标签编辑距离选出最近的 num 个用户。
// 在线 miss 与预热均调用此函数，保证同一 candidates 下结果一致。
func (s *Service) matchUsersFromCandidates(ctx context.Context, num int, loginUser *User, candidates []*User) ([]*User, error) {
	loginTags := loginUser.ParseTags()

	scoredCandidates := make([]scoredUser, 0, len(candidates))
	for _, u := range candidates {
		if u.ID == loginUser.ID {
			continue
		}
		uTags := u.ParseTags()
		if len(uTags) == 0 {
			continue
		}
		scoredCandidates = append(scoredCandidates, scoredUser{
			userID:   u.ID,
			distance: minDistance(loginTags, uTags),
		})
	}

	ids := topKNearest(scoredCandidates, num)

	matched, err := s.repo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]*User, len(matched))
	for _, u := range matched {
		byID[u.ID] = u
	}

	result := make([]*User, 0, len(ids))
	for _, id := range ids {
		if u, ok := byID[id]; ok {
			result = append(result, u)
		}
	}
	return result, nil
}

// isAdmin 判断用户是否为管理员
func (s *Service) isAdmin(u *User) bool {
	return u != nil && u.UserRole == RoleAdmin
}

func containsAll(userTags []string, required map[string]struct{}) bool {
	have := make(map[string]struct{}, len(userTags))
	for _, t := range userTags {
		have[t] = struct{}{}
	}
	for tag := range required {
		if _, ok := have[tag]; !ok {
			return false
		}
	}
	return true
}
