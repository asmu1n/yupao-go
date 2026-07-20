package user

import (
	"context"
	"regexp"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"yupao-go/internal/pkg/response"
	"yupao-go/internal/port"
)

var validAccountPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

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

func (s *Service) ListByIDs(ctx context.Context, ids []int64) ([]*User, error) {
	list, err := s.repo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, response.NewBizError(response.NotFound)
	}
	return list, nil
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
	isAdmin := s.IsAdmin(caller)

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

// IsAdmin 判断用户是否为管理员
func (s *Service) IsAdmin(u *User) bool {
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
