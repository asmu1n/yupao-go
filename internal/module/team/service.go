package team

import (
	"context"
	"fmt"
	"time"

	"yupao-go/internal/module/user"
	"yupao-go/internal/pkg/logger"
	"yupao-go/internal/pkg/page"
	"yupao-go/internal/pkg/response"
	"yupao-go/internal/pkg/types"
	"yupao-go/internal/port"
)

// joinLockKey 按队伍互斥，避免并发 Join 超员/同队重复加入；不同 team 互不阻塞。
// 同一用户跨多队并发导致「最多 5 队」的竞态概率低，暂不按 user 加锁。
func joinLockKey(teamID int64) string {
	return fmt.Sprintf("yupao:join_team:%d", teamID)
}

const joinLockTTL = 10 * time.Second

// teamLog 队伍模块业务/审计日志。
var teamLog = logger.Module("team")

// Service 队伍用例服务。
type Service struct {
	repo   Repository
	users  UserReader
	locker port.Locker
}

// UserReader 读取用户信息（创建人展示、管理员判断）。
type UserReader interface {
	ListByIDs(ctx context.Context, ids []int64) ([]*user.User, error)
	IsAdmin(u *user.User) bool
}

// NewService 构造队伍服务。
func NewService(repo Repository, users UserReader, locker port.Locker) *Service {
	return &Service{repo: repo, users: users, locker: locker}
}

// Add 创建队伍并自动加入队长。
func (s *Service) Add(ctx context.Context, p AddParams, actionUserID int64) (int64, error) {
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

	createdCount, err := s.repo.CountCreatedByUser(ctx, actionUserID)
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
		UserID:      actionUserID,
		Status:      status,
		Password:    password,
	}
	id, err := s.repo.CreateTeamWithLeader(ctx, t, actionUserID)
	if err != nil {
		return 0, err
	}
	teamLog.Info("team created",
		logger.FieldPurpose, logger.PurposeAudit,
		logger.FieldEvent, "team.created",
		"team_id", id,
		"user_id", actionUserID,
		"status", status,
		"max_num", p.MaxNum,
	)
	return id, nil
}

// GetByID 按 ID 获取队伍（不含密码）。
func (s *Service) GetByID(ctx context.Context, id, viewerID int64, isAdmin bool) (*TeamUserVO, error) {
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
	isVisible, err := s.canViewTeam(ctx, t, viewerID, isAdmin)
	if err != nil {
		return nil, err
	}
	if !isVisible {
		return nil, response.NewBizErrorWithDetail(response.NotFound, "队伍不存在")
	}
	teamVO, err := s.toTeamUserVOs(ctx, []*Team{t}, viewerID)
	if err != nil {
		return nil, err
	}
	return teamVO[0], nil
}

// List 查询队伍列表并填充创建人、加入态与人数。
func (s *Service) List(ctx context.Context, q QueryParams, viewerID int64, isAdmin bool) ([]*TeamUserVO, error) {
	if !isAdmin && q.Status != nil && *q.Status == types.TeamStatusPrivate {
		return nil, response.NewBizError(response.NoAuth)
	}

	teams, err := s.repo.List(ctx, q, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.toTeamUserVOs(ctx, teams, viewerID)
}

// ListPage 分页查询队伍实体（管理/简单列表，不含 VO  enrichment）。
func (s *Service) ListPage(ctx context.Context, q QueryParams, viewerID int64, isAdmin bool) (*page.PageResponse[*TeamUserVO], error) {
	if !isAdmin && q.Status != nil && *q.Status == types.TeamStatusPrivate {
		return nil, response.NewBizError(response.NoAuth)
	}
	rows, total, err := s.repo.ListPage(ctx, q, isAdmin)
	if err != nil {
		return nil, err
	}
	rowsVO, err := s.toTeamUserVOs(ctx, rows, viewerID)
	if err != nil {
		return nil, err
	}
	return page.NewPageResponse(rowsVO, total, q.PageRequest), nil
}

// ListMyCreate 检索当前用户创建的队伍。
func (s *Service) ListMyCreate(ctx context.Context, q MyCreateQueryParams, viewerID int64) ([]*TeamUserVO, error) {
	query := QueryParams{
		SearchText:  q.SearchText,
		Name:        q.Name,
		Description: q.Description,
		MaxNum:      q.MaxNum,
		OwnerID:     &viewerID,
		Status:      q.Status,
	}
	teams, err := s.repo.List(ctx, query, true)
	if err != nil {
		return nil, err
	}
	return s.toTeamUserVOs(ctx, teams, viewerID)
}

// ListMyJoin 检索当前用户加入的队伍。
func (s *Service) ListMyJoin(ctx context.Context, q MyJoinQueryParams, viewerID int64) ([]*TeamUserVO, error) {
	query := QueryParams{
		SearchText:  q.SearchText,
		Name:        q.Name,
		Description: q.Description,
		MaxNum:      q.MaxNum,
		Status:      q.Status,
	}
	// 查询目标加入哪些队伍
	ids, err := s.repo.ListTeamIDsByUser(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []*TeamUserVO{}, nil
	}
	query.IDList = append([]int64(nil), ids...)
	teams, err := s.repo.List(ctx, query, true)
	if err != nil {
		return nil, err
	}
	return s.toTeamUserVOs(ctx, teams, viewerID)
}

// Update 更新队伍（队长或管理员）。
func (s *Service) Update(ctx context.Context, p UpdateParams, actionUserID int64, isAdmin bool) error {
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
	if old.UserID != actionUserID && !isAdmin {
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

	if err := s.repo.Update(ctx, old); err != nil {
		return err
	}
	teamLog.Info("team updated",
		logger.FieldPurpose, logger.PurposeBiz,
		logger.FieldEvent, "team.updated",
		"team_id", p.ID,
		"user_id", actionUserID,
		"is_admin", isAdmin,
	)
	return nil
}

// Join 加入队伍（分布式锁防止并发超员/重复加入）。
func (s *Service) Join(ctx context.Context, p JoinParams, actionUserID int64) error {
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
		joined, err := s.repo.CountUserMemberships(ctx, actionUserID)
		if err != nil {
			return err
		}
		if joined >= maxJoinTeams {
			return response.NewBizErrorWithDetail(response.ParamsError, "最多创建和加入 5 个队伍")
		}
		has, err := s.repo.HasJoined(ctx, actionUserID, p.TeamID)
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
		return s.repo.AddMember(ctx, actionUserID, p.TeamID, time.Now())
	}

	if s.locker == nil {
		if err := doJoin(); err != nil {
			return err
		}
		teamLog.Info("team joined",
			logger.FieldPurpose, logger.PurposeAudit,
			logger.FieldEvent, "team.joined",
			"team_id", p.TeamID,
			"user_id", actionUserID,
		)
		return nil
	}
	err = s.locker.RunWithLock(ctx, joinLockKey(p.TeamID), joinLockTTL, doJoin)
	if err == port.ErrLockFailed {
		teamLog.Warn("team join busy",
			logger.FieldPurpose, logger.PurposeBiz,
			logger.FieldEvent, "team.join_busy",
			"team_id", p.TeamID,
			"user_id", actionUserID,
		)
		return response.NewBizErrorWithDetail(response.SystemError, "系统繁忙，请稍后重试")
	}
	if err != nil {
		return err
	}
	teamLog.Info("team joined",
		logger.FieldPurpose, logger.PurposeAudit,
		logger.FieldEvent, "team.joined",
		"team_id", p.TeamID,
		"user_id", actionUserID,
	)
	return nil
}

// Quit 退出队伍；事务内锁定 team 行，原子处理退出 / 最后一人解散 / 队长移交。
func (s *Service) Quit(ctx context.Context, p QuitParams, targetUserID int64) error {
	if p.TeamID <= 0 {
		return response.NewBizError(response.ParamsError)
	}

	result, err := s.repo.QuitMember(ctx, p.TeamID, targetUserID)
	if err != nil {
		// 移交失败等系统级业务错误打 alert，其余 BizError 直接上抛
		if be, ok := err.(*response.BizError); ok && be.BizCode() == response.SystemError.Biz {
			teamLog.Error("transfer leader failed",
				logger.FieldPurpose, logger.PurposeAlert,
				logger.FieldEvent, "team.transfer_leader_error",
				"team_id", p.TeamID,
				"user_id", targetUserID,
				"err", err.Error(),
			)
		}
		return err
	}

	switch result.Outcome {
	case QuitOutcomeDisbanded:
		teamLog.Info("team disbanded on quit",
			logger.FieldPurpose, logger.PurposeAudit,
			logger.FieldEvent, "team.disbanded",
			"team_id", p.TeamID,
			"user_id", targetUserID,
			"reason", "last_member_quit",
		)
	case QuitOutcomeTransferred:
		teamLog.Info("leader quit and transferred",
			logger.FieldPurpose, logger.PurposeAudit,
			logger.FieldEvent, "team.leader_transferred",
			"team_id", p.TeamID,
			"user_id", targetUserID,
			"new_leader_id", result.NewLeaderID,
		)
	default:
		teamLog.Info("team quit",
			logger.FieldPurpose, logger.PurposeAudit,
			logger.FieldEvent, "team.quit",
			"team_id", p.TeamID,
			"user_id", targetUserID,
		)
	}
	return nil
}

// Delete 队长解散队伍（事务内锁定 team 并校验队长身份）。
func (s *Service) Delete(ctx context.Context, teamID, actionUserID int64) error {
	if teamID <= 0 {
		return response.NewBizError(response.ParamsError)
	}
	if err := s.repo.SoftDeleteTeamAndMembersByLeader(ctx, teamID, actionUserID); err != nil {
		return err
	}
	teamLog.Info("team deleted",
		logger.FieldPurpose, logger.PurposeAudit,
		logger.FieldEvent, "team.deleted",
		"team_id", teamID,
		"user_id", actionUserID,
	)
	return nil
}

func (s *Service) canViewTeam(ctx context.Context, t *Team, viewerID int64, isAdmin bool) (bool, error) {
	// 如果不是私有队伍就开放查看权限
	if t.Status != types.TeamStatusPrivate {
		return true, nil
	}
	// 检查用户类型，（所有者、管理员 /  游客）
	if isAdmin || t.UserID == viewerID {
		return true, nil
	} else if viewerID == 0 {
		return false, nil
	}

	return s.repo.HasJoined(ctx, viewerID, t.ID)
}

func (s *Service) toTeamUserVOs(ctx context.Context, teams []*Team, viewerID int64) ([]*TeamUserVO, error) {
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

	joined := map[int64]struct{}{}
	if viewerID > 0 {
		joined, err = s.repo.JoinedTeamIDs(ctx, viewerID, ids)
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
		u := userMap[t.UserID]
		if u != nil {
			vo.CreateUser = toTeamCreatorVO(u)
		}
		out = append(out, vo)
	}
	return out, nil
}

func toTeamCreatorVO(u *user.User) *TeamCreatorVO {
	if u == nil {
		return nil
	}
	return &TeamCreatorVO{
		ID:        u.ID,
		Username:  u.Username,
		AvatarURL: u.AvatarURL,
		Gender:    u.Gender,
	}
}
