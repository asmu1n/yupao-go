package team

import (
	"context"
	"time"

	"yupao-go/internal/module/user"
	"yupao-go/internal/pkg/page"
	"yupao-go/internal/pkg/response"
	"yupao-go/internal/pkg/types"
	"yupao-go/internal/port"
)

const joinLockKey = "yupao:join_team"
const joinLockTTL = 10 * time.Second

// Service 队伍用例服务。
type Service struct {
	repo   Repository
	users  UserReader
	locker port.Locker
}

// UserReader 读取用户信息（创建人展示、管理员判断）。
type UserReader interface {
	ListByIDs(ctx context.Context, ids []int64) ([]*user.User, error)
}

// NewService 构造队伍服务。
func NewService(repo Repository, users UserReader, locker port.Locker) *Service {
	return &Service{repo: repo, users: users, locker: locker}
}

// Add 创建队伍并自动加入队长。
func (s *Service) Add(ctx context.Context, p AddParams, loginUserID int64) (int64, error) {
	if loginUserID <= 0 {
		return 0, response.NewBizError(response.NotLogin)
	}
	if p.MaxNum < minTeamMembers || p.MaxNum > maxTeamMembers {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "队伍人数不满足要求")
	}
	if p.Name == "" || len([]rune(p.Name)) > 20 {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "队伍标题不满足要求")
	}
	if p.Description != nil && len([]rune(*p.Description)) > 512 {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "队伍描述过长")
	}

	status := types.TeamStatusPublic
	if p.Status != nil {
		status = *p.Status
	}
	if !status.Valid() {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "队伍状态不满足要求")
	}
	password := ""
	if status == types.TeamStatusSecret {
		if p.Password == nil || *p.Password == "" || len(*p.Password) > 32 {
			return 0, response.NewBizErrorWithDetail(response.ParamsError, "密码设置不正确")
		}
		password = *p.Password
	}
	if p.ExpireTime == nil || !p.ExpireTime.After(time.Now()) {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "超时时间必须晚于当前时间")
	}

	createdCount, err := s.repo.CountCreatedByUser(ctx, loginUserID)
	if err != nil {
		return 0, err
	}
	if createdCount >= maxCreateTeams {
		return 0, response.NewBizErrorWithDetail(response.ParamsError, "用户最多创建 5 个队伍")
	}

	t := &Team{
		Name:        p.Name,
		Description: p.Description,
		MaxNum:      p.MaxNum,
		ExpireTime:  p.ExpireTime,
		UserID:      loginUserID,
		Status:      status,
		Password:    password,
	}
	return s.repo.CreateTeamWithLeader(ctx, t, loginUserID)
}

// GetByID 按 ID 获取队伍（不含密码）。
func (s *Service) GetByID(ctx context.Context, id int64) (*Team, error) {
	if id <= 0 {
		return nil, response.NewBizError(response.ParamsError)
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "队伍不存在")
	}
	return t, nil
}

// List 查询队伍列表并填充创建人、加入态与人数。
// loginUserID 为 0 表示未登录（不填充 hasJoin）。
func (s *Service) List(ctx context.Context, q QueryParams, loginUserID int64, isAdmin bool) ([]*TeamUserVO, error) {
	status := types.TeamStatusPublic
	if q.Status != nil {
		status = *q.Status
	} else {
		q.Status = &status
	}
	if !status.Valid() {
		status = types.TeamStatusPublic
		q.Status = &status
	}
	if !isAdmin && status == types.TeamStatusPrivate {
		return nil, response.NewBizError(response.NoAuth)
	}

	teams, err := s.repo.List(ctx, q, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.toTeamUserVOs(ctx, teams, loginUserID)
}

func (s *Service) ListPage(ctx context.Context, q QueryParams) (*page.PageResponse[*Team], error) {
	rows, total, err := s.repo.ListPage(ctx, q)
	if err != nil {
		return nil, err
	}
	return page.NewPageResponse(rows, total, q.PageRequest), nil
}

// 检索自己创建的队伍
func (s *Service) ListMyCreate(ctx context.Context, q QueryParams, loginUserID int64) ([]*TeamUserVO, error) {
	if loginUserID <= 0 {
		return nil, response.NewBizError(response.NotLogin)
	}
	uid := loginUserID
	q.UserID = &uid
	// 创建者视角可看自己创建的私有队
	teams, err := s.repo.List(ctx, q, true)
	if err != nil {
		return nil, err
	}
	return s.toTeamUserVOs(ctx, teams, loginUserID)
}

// 检索自己所属的队伍
func (s *Service) ListMyJoin(ctx context.Context, q QueryParams, loginUserID int64) ([]*TeamUserVO, error) {
	if loginUserID <= 0 {
		return nil, response.NewBizError(response.NotLogin)
	}
	ids, err := s.repo.ListTeamIDsByUser(ctx, loginUserID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []*TeamUserVO{}, nil
	}
	q.IDList = ids
	teams, err := s.repo.List(ctx, q, true)
	if err != nil {
		return nil, err
	}
	return s.toTeamUserVOs(ctx, teams, loginUserID)
}

// Update 更新队伍（队长或管理员）。
func (s *Service) Update(ctx context.Context, p UpdateParams, loginUserID int64, isAdmin bool) error {
	if p.ID <= 0 {
		return response.NewBizError(response.ParamsError)
	}
	old, err := s.repo.GetByID(ctx, p.ID)
	if err != nil {
		return err
	}
	if old == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "队伍不存在")
	}
	if old.UserID != loginUserID && !isAdmin {
		return response.NewBizError(response.NoAuth)
	}

	if p.Name != nil {
		if *p.Name == "" || len([]rune(*p.Name)) > 20 {
			return response.NewBizErrorWithDetail(response.ParamsError, "队伍标题不满足要求")
		}
		old.Name = *p.Name
	}
	if p.Description != nil {
		if len([]rune(*p.Description)) > 512 {
			return response.NewBizErrorWithDetail(response.ParamsError, "队伍描述过长")
		}
		old.Description = p.Description
	}
	if p.ExpireTime != nil {
		old.ExpireTime = p.ExpireTime
	}
	if p.Status != nil {
		if !p.Status.Valid() {
			return response.NewBizErrorWithDetail(response.ParamsError, "队伍状态不满足要求")
		}
		old.Status = *p.Status
	}
	if old.Status == types.TeamStatusSecret {
		if p.Password != nil {
			if *p.Password == "" {
				return response.NewBizErrorWithDetail(response.ParamsError, "加密房间必须要设置密码")
			}
			old.Password = *p.Password
		} else if old.Password == "" {
			return response.NewBizErrorWithDetail(response.ParamsError, "加密房间必须要设置密码")
		}
	} else if p.Password != nil {
		old.Password = *p.Password
	}

	return s.repo.Update(ctx, old)
}

// Join 加入队伍（分布式锁防止并发超员/重复加入）。
func (s *Service) Join(ctx context.Context, p JoinParams, loginUserID int64) error {
	if loginUserID <= 0 {
		return response.NewBizError(response.NotLogin)
	}
	t, err := s.repo.GetByID(ctx, p.TeamID)
	if err != nil {
		return err
	}
	if t == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "队伍不存在")
	}
	if t.ExpireTime != nil && t.ExpireTime.Before(time.Now()) {
		return response.NewBizErrorWithDetail(response.ParamsError, "队伍已过期")
	}
	if t.Status == types.TeamStatusPrivate {
		return response.NewBizErrorWithDetail(response.ParamsError, "禁止加入私有队伍")
	}
	if t.Status == types.TeamStatusSecret {
		if p.Password == nil || *p.Password != t.Password {
			return response.NewBizErrorWithDetail(response.ParamsError, "密码错误")
		}
	}

	doJoin := func() error {
		joined, err := s.repo.CountUserMemberships(ctx, loginUserID)
		if err != nil {
			return err
		}
		if joined >= maxJoinTeams {
			return response.NewBizErrorWithDetail(response.ParamsError, "最多创建和加入 5 个队伍")
		}
		has, err := s.repo.HasJoined(ctx, loginUserID, p.TeamID)
		if err != nil {
			return err
		}
		if has {
			return response.NewBizErrorWithDetail(response.ParamsError, "用户已加入该队伍")
		}
		n, err := s.repo.CountMembers(ctx, p.TeamID)
		if err != nil {
			return err
		}
		if n >= int64(t.MaxNum) {
			return response.NewBizErrorWithDetail(response.ParamsError, "队伍已满")
		}
		return s.repo.AddMember(ctx, loginUserID, p.TeamID, time.Now())
	}

	if s.locker == nil {
		return doJoin()
	}
	err = s.locker.RunWithLock(ctx, joinLockKey, joinLockTTL, doJoin)
	if err == port.ErrLockFailed {
		return response.NewBizErrorWithDetail(response.SystemError, "系统繁忙，请稍后重试")
	}
	return err
}

// Quit 退出队伍；仅剩一人时解散；队长退出则移交最早加入的成员。
func (s *Service) Quit(ctx context.Context, p QuitParams, loginUserID int64) error {
	if loginUserID <= 0 {
		return response.NewBizError(response.NotLogin)
	}
	t, err := s.repo.GetByID(ctx, p.TeamID)
	if err != nil {
		return err
	}
	if t == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "队伍不存在")
	}
	has, err := s.repo.HasJoined(ctx, loginUserID, p.TeamID)
	if err != nil {
		return err
	}
	if !has {
		return response.NewBizErrorWithDetail(response.ParamsError, "未加入队伍")
	}

	n, err := s.repo.CountMembers(ctx, p.TeamID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return s.repo.SoftDeleteTeamAndMembers(ctx, p.TeamID)
	}

	if t.UserID == loginUserID {
		// 移交队长：按成员关系 id 升序，取第一个非当前队长的成员（最早加入者优先）
		members, err := s.repo.ListMembersByTeamOrdered(ctx, p.TeamID, 0)
		if err != nil {
			return err
		}
		var next int64
		for _, m := range members {
			if m.UserID != loginUserID {
				next = m.UserID
				break
			}
		}
		if next == 0 {
			return response.NewBizError(response.SystemError)
		}
		return s.repo.TransferLeaderAndRemoveMember(ctx, p.TeamID, loginUserID, next)
	}

	return s.repo.SoftDeleteMember(ctx, loginUserID, p.TeamID)
}

// Delete 队长解散队伍。
func (s *Service) Delete(ctx context.Context, teamID, loginUserID int64) error {
	if teamID <= 0 {
		return response.NewBizError(response.ParamsError)
	}
	t, err := s.repo.GetByID(ctx, teamID)
	if err != nil {
		return err
	}
	if t == nil {
		return response.NewBizErrorWithDetail(response.NotFound, "队伍不存在")
	}
	if t.UserID != loginUserID {
		return response.NewBizErrorWithDetail(response.NoAuth, "无访问权限")
	}
	return s.repo.SoftDeleteTeamAndMembers(ctx, teamID)
}

// IsAdminUser 根据用户角色判断是否管理员。
func IsAdminUser(u *user.User) bool {
	return u != nil && u.UserRole == user.RoleAdmin
}

func (s *Service) toTeamUserVOs(ctx context.Context, teams []*Team, loginUserID int64) ([]*TeamUserVO, error) {
	if len(teams) == 0 {
		return []*TeamUserVO{}, nil
	}

	// 收集所有需要查询的用户ID以及队伍ID
	ids := make([]int64, len(teams))
	userMap := make(map[int64]*user.User)
	for i, t := range teams {
		ids[i] = t.ID
		if t.UserID > 0 {
			userMap[t.UserID] = nil
		}
	}

	// 借助 map 去重用户ID
	userIds := make([]int64, 0, len(userMap))
	for uid := range userMap {
		userIds = append(userIds, uid)
	}

	counts, err := s.repo.CountMembersByTeamIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	// 批量查询用户信息，同时复用 userMap 录入数据
	users, err := s.users.ListByIDs(ctx, userIds)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		userMap[u.ID] = u
	}

	var joined map[int64]struct{}
	if loginUserID > 0 {
		joined, err = s.repo.JoinedTeamIDs(ctx, loginUserID, ids)
		if err != nil {
			return nil, err
		}
	}

	out := make([]*TeamUserVO, 0, len(teams))
	for _, t := range teams {
		vo := &TeamUserVO{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			MaxNum:      t.MaxNum,
			ExpireTime:  t.ExpireTime,
			UserID:      t.UserID,
			Status:      t.Status,
			CreateTime:  t.CreateTime,
			UpdateTime:  t.UpdateTime,
			HasJoinNum:  counts[t.ID],
		}
		if joined != nil {
			_, vo.HasJoin = joined[t.ID]
		}
		if t.UserID > 0 {
			u := userMap[t.UserID]
			if u != nil {
				// Password 已 json:"-"，可直接返回
				vo.CreateUser = u
			}
		}
		out = append(out, vo)
	}
	return out, nil
}
