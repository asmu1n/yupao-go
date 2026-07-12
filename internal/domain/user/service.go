package user

import (
	"context"
	"regexp"
	"sort"

	"golang.org/x/crypto/bcrypt"

	"yupao-go/internal/core"
	"yupao-go/internal/pkg/algo"
)

var validAccountPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Register 用户注册，校验参数 + 查重 + bcrypt 加密后入库，返回新用户 ID
func (s *Service) Register(ctx context.Context, p RegisterParams) (int64, error) {
	if !validAccountPattern.MatchString(p.UserAccount) {
		return 0, core.NewBizErrorWithDetail(core.ParamsError, "账号包含特殊字符")
	}

	exists, err := s.repo.ExistsByAccount(ctx, p.UserAccount)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, core.NewBizErrorWithDetail(core.ParamsError, "账号重复")
	}

	exists, err = s.repo.ExistsByPlanetCode(ctx, p.PlanetCode)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, core.NewBizErrorWithDetail(core.ParamsError, "编号重复")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(p.UserPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, core.NewBizErrorWithDetail(core.SystemError, "密码加密失败")
	}

	u := &User{
		UserAccount: &p.UserAccount,
		Password:    string(hashed),
		PlanetCode:  &p.PlanetCode,
	}
	return s.repo.Create(ctx, u)
}

// Login 用户登录，校验账号密码后返回脱敏用户信息
func (s *Service) Login(ctx context.Context, account, password string) (*User, error) {
	if !validAccountPattern.MatchString(account) {
		return nil, core.NewBizErrorWithDetail(core.ParamsError, "账号包含特殊字符")
	}

	u, err := s.repo.GetByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, core.NewBizErrorWithDetail(core.ParamsError, "账号或密码错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, core.NewBizErrorWithDetail(core.ParamsError, "账号或密码错误")
	}

	u.Password = ""
	return u, nil
}

// GetByID 根据 ID 获取脱敏用户信息
func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, core.NewBizError(core.NotFound)
	}
	u.Password = ""
	return u, nil
}

// Update 更新用户信息，管理员可改任意用户，普通用户仅可改自己
func (s *Service) Update(ctx context.Context, targetID int64, u *User, callerID int64) error {
	caller, err := s.repo.GetByID(ctx, callerID)
	if err != nil {
		return err
	}
	if caller == nil {
		return core.NewBizError(core.NotFound)
	}
	isAdmin := s.isAdmin(caller)

	if targetID <= 0 {
		return core.NewBizError(core.ParamsError)
	}
	if !isAdmin && targetID != callerID {
		return core.NewBizError(core.NoAuth)
	}

	old, err := s.repo.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if old == nil {
		return core.NewBizError(core.NotFound)
	}

	return s.repo.Update(ctx, targetID, u)
}

// SearchByTags 根据标签列表搜索用户，内存过滤匹配所有标签的用户
func (s *Service) SearchByTags(ctx context.Context, tagNames []string) ([]*User, error) {
	if len(tagNames) == 0 {
		return nil, core.NewBizError(core.ParamsError)
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
			u.Password = ""
			result = append(result, u)
		}
	}
	return result, nil
}

// MatchUsers 基于标签编辑距离推荐最相似的 num 个用户
func (s *Service) MatchUsers(ctx context.Context, num int, loginUser *User) ([]*User, error) {
	users, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	loginTags := loginUser.ParseTags()
	if len(loginTags) == 0 {
		return nil, nil
	}

	type pair struct {
		userID   int64
		distance int
	}

	var pairs []pair
	for _, u := range users {
		if u.ID == loginUser.ID {
			continue
		}
		uTags := u.ParseTags()
		if len(uTags) == 0 {
			continue
		}
		dist := algo.MinDistance(loginTags, uTags)
		pairs = append(pairs, pair{userID: u.ID, distance: dist})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].distance < pairs[j].distance
	})
	if len(pairs) > num {
		pairs = pairs[:num]
	}

	ids := make([]int64, len(pairs))
	for i, p := range pairs {
		ids[i] = p.userID
	}

	matched, err := s.repo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]*User, len(matched))
	for _, u := range matched {
		u.Password = ""
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
