package repo

import (
	"context"
	"fmt"
	"time"

	"yupao-go/ent"
	"yupao-go/ent/predicate"
	entteam "yupao-go/ent/team"
	entuserteam "yupao-go/ent/userteam"
	"yupao-go/internal/module/team"
	"yupao-go/internal/pkg/types"
)

// EntRepository 基于 ent 的队伍仓储。
type EntRepository struct {
	client *ent.Client
}

// New 构造队伍仓储。
func New(client *ent.Client) *EntRepository {
	return &EntRepository{client: client}
}

func (r *EntRepository) CreateTeamWithLeader(ctx context.Context, t *team.Team, leaderID int64) (int64, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	builder := tx.Team.Create().
		SetName(t.Name).
		SetMaxNum(t.MaxNum).
		SetUserID(leaderID).
		SetStatus(t.Status)
	if t.Description != nil {
		builder = builder.SetNillableDescription(t.Description)
	}
	if t.ExpireTime != nil {
		builder = builder.SetNillableExpireTime(t.ExpireTime)
	}
	if t.Password != "" {
		builder = builder.SetPassword(t.Password)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	_, err = tx.UserTeam.Create().
		SetUserID(leaderID).
		SetTeamID(created.ID).
		SetJoinTime(now).
		Save(ctx)
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *EntRepository) GetByID(ctx context.Context, id int64) (*team.Team, error) {
	row, err := r.client.Team.Query().
		Where(entteam.IDEQ(id), entteam.IsDeleteEQ(0)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *EntRepository) Update(ctx context.Context, t *team.Team) error {
	// 仅更新未删除队伍
	exists, err := r.client.Team.Query().
		Where(entteam.IDEQ(t.ID), entteam.IsDeleteEQ(0)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return &ent.NotFoundError{}
	}

	upd := r.client.Team.UpdateOneID(t.ID).
		SetName(t.Name).
		SetMaxNum(t.MaxNum).
		SetUserID(t.UserID).
		SetStatus(t.Status)
	if t.Description != nil {
		upd = upd.SetNillableDescription(t.Description)
	} else {
		upd = upd.ClearDescription()
	}
	if t.ExpireTime != nil {
		upd = upd.SetNillableExpireTime(t.ExpireTime)
	} else {
		upd = upd.ClearExpireTime()
	}
	if t.Password != "" {
		upd = upd.SetPassword(t.Password)
	} else {
		upd = upd.ClearPassword()
	}
	_, err = upd.Save(ctx)
	return err
}

func (r *EntRepository) SoftDeleteTeam(ctx context.Context, id int64) error {
	_, err := r.client.Team.UpdateOneID(id).
		SetIsDelete(1).
		Save(ctx)
	return err
}

func (r *EntRepository) SoftDeleteTeamAndMembers(ctx context.Context, teamID int64) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.UserTeam.Update().
		Where(entuserteam.TeamIDEQ(teamID), entuserteam.IsDeleteEQ(0)).
		SetIsDelete(1).
		Save(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Team.UpdateOneID(teamID).
		SetIsDelete(1).
		Save(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *EntRepository) CountCreatedByUser(ctx context.Context, userID int64) (int64, error) {
	n, err := r.client.Team.Query().
		Where(entteam.UserIDEQ(userID), entteam.IsDeleteEQ(0)).
		Count(ctx)
	return int64(n), err
}

func (r *EntRepository) List(ctx context.Context, q team.QueryParams, includePrivate bool) ([]*team.Team, error) {
	query := r.client.Team.Query().Where(buildListPreds(q, includePrivate)...)
	rows, err := query.Order(ent.Desc(entteam.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return toDomainList(rows), nil
}

func (r *EntRepository) ListPage(ctx context.Context, q team.QueryParams, includePrivate bool) ([]*team.Team, int64, error) {
	preds := buildListPreds(q, includePrivate)
	query := r.client.Team.Query().Where(preds...)
	total, err := r.client.Team.Query().Where(preds...).Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := query.
		Offset(q.Offset()).
		Limit(q.Limit()).
		Order(ent.Desc(entteam.FieldID)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return toDomainList(rows), int64(total), nil
}

func buildListPreds(q team.QueryParams, includePrivate bool) []predicate.Team {
	preds := []predicate.Team{entteam.IsDeleteEQ(0)}

	// 未过期：expire_time IS NULL OR expire_time > now
	now := time.Now()
	preds = append(preds, entteam.Or(
		entteam.ExpireTimeIsNil(),
		entteam.ExpireTimeGT(now),
	))

	if q.ID != nil && *q.ID > 0 {
		preds = append(preds, entteam.IDEQ(*q.ID))
	}
	if len(q.IDList) > 0 {
		preds = append(preds, entteam.IDIn(q.IDList...))
	}
	if q.SearchText != "" {
		preds = append(preds, entteam.Or(
			entteam.NameContains(q.SearchText),
			entteam.DescriptionContains(q.SearchText),
		))
	}
	if q.Name != "" {
		preds = append(preds, entteam.NameContains(q.Name))
	}
	if q.Description != "" {
		preds = append(preds, entteam.DescriptionContains(q.Description))
	}
	if q.MaxNum != nil && *q.MaxNum > 0 {
		preds = append(preds, entteam.MaxNumEQ(*q.MaxNum))
	}
	if q.OwnerID != nil && *q.OwnerID > 0 {
		preds = append(preds, entteam.UserIDEQ(*q.OwnerID))
	}

	// status 为 nil 时，默认仅返回公开 / 加密；显式允许时才包含私有。
	if q.Status != nil {
		status := *q.Status
		if !includePrivate && status == types.TeamStatusPrivate {
			preds = append(preds, entteam.IDEQ(0))
		} else {
			preds = append(preds, entteam.StatusEQ(status))
		}
	} else if !includePrivate {
		preds = append(preds, entteam.StatusIn(types.TeamStatusPublic, types.TeamStatusSecret))
	}

	return preds
}

func (r *EntRepository) AddMember(ctx context.Context, userID, teamID int64, joinTime time.Time) error {
	_, err := r.client.UserTeam.Create().
		SetUserID(userID).
		SetTeamID(teamID).
		SetJoinTime(joinTime).
		Save(ctx)
	return err
}

func (r *EntRepository) SoftDeleteMember(ctx context.Context, userID, teamID int64) error {
	n, err := r.client.UserTeam.Update().
		Where(
			entuserteam.UserIDEQ(userID),
			entuserteam.TeamIDEQ(teamID),
			entuserteam.IsDeleteEQ(0),
		).
		SetIsDelete(1).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}

func (r *EntRepository) CountMembers(ctx context.Context, teamID int64) (int64, error) {
	n, err := r.client.UserTeam.Query().
		Where(entuserteam.TeamIDEQ(teamID), entuserteam.IsDeleteEQ(0)).
		Count(ctx)
	return int64(n), err
}

func (r *EntRepository) CountUserMemberships(ctx context.Context, userID int64) (int64, error) {
	n, err := r.client.UserTeam.Query().
		Where(entuserteam.UserIDEQ(userID), entuserteam.IsDeleteEQ(0)).
		Count(ctx)
	return int64(n), err
}

func (r *EntRepository) HasJoined(ctx context.Context, userID, teamID int64) (bool, error) {
	return r.client.UserTeam.Query().
		Where(
			entuserteam.UserIDEQ(userID),
			entuserteam.TeamIDEQ(teamID),
			entuserteam.IsDeleteEQ(0),
		).
		Exist(ctx)
}

func (r *EntRepository) CountMembersByTeamIDs(ctx context.Context, teamIDs []int64) (map[int64]int, error) {
	// 维护字典表，key为teamID，value为成员数量
	out := make(map[int64]int, len(teamIDs))
	if len(teamIDs) == 0 {
		return out, nil
	}
	// 查询所有 team 的成员情况
	rows, err := r.client.UserTeam.Query().
		Where(entuserteam.TeamIDIn(teamIDs...), entuserteam.IsDeleteEQ(0)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	// 统计每个 team 的成员数量
	for _, row := range rows {
		out[row.TeamID]++
	}
	return out, nil
}

func (r *EntRepository) JoinedTeamIDs(ctx context.Context, userID int64, teamIDs []int64) (map[int64]struct{}, error) {
	out := make(map[int64]struct{})
	if len(teamIDs) == 0 {
		return out, nil
	}
	rows, err := r.client.UserTeam.Query().
		Where(
			entuserteam.UserIDEQ(userID),
			entuserteam.TeamIDIn(teamIDs...),
			entuserteam.IsDeleteEQ(0),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.TeamID] = struct{}{}
	}
	return out, nil
}

func (r *EntRepository) ListTeamIDsByUser(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.client.UserTeam.Query().
		Where(entuserteam.UserIDEQ(userID), entuserteam.IsDeleteEQ(0)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	// 去重
	seen := make(map[int64]struct{}, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.TeamID]; ok {
			continue
		}
		seen[row.TeamID] = struct{}{}
		ids = append(ids, row.TeamID)
	}
	return ids, nil
}

func (r *EntRepository) ListMembersByTeamOrdered(ctx context.Context, teamID int64, limit int) ([]team.Member, error) {
	q := r.client.UserTeam.Query().
		Where(entuserteam.TeamIDEQ(teamID), entuserteam.IsDeleteEQ(0)).
		Order(ent.Asc(entuserteam.FieldID))
	if limit > 0 {
		q = q.Limit(limit)
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]team.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, team.Member{ID: row.ID, UserID: row.UserID, TeamID: row.TeamID})
	}
	return out, nil
}

func (r *EntRepository) TransferLeaderAndRemoveMember(ctx context.Context, teamID, oldLeaderID, newLeaderID int64) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.Team.UpdateOneID(teamID).
		SetUserID(newLeaderID).
		Save(ctx)
	if err != nil {
		return err
	}
	_, err = tx.UserTeam.Update().
		Where(
			entuserteam.UserIDEQ(oldLeaderID),
			entuserteam.TeamIDEQ(teamID),
			entuserteam.IsDeleteEQ(0),
		).
		SetIsDelete(1).
		Save(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func toDomain(e *ent.Team) *team.Team {
	t := &team.Team{
		ID:         e.ID,
		Name:       e.Name,
		MaxNum:     e.MaxNum,
		UserID:     e.UserID,
		Status:     e.Status,
		CreateTime: e.CreateTime,
		UpdateTime: e.UpdateTime,
	}
	if e.Description != nil {
		t.Description = e.Description
	}
	if e.ExpireTime != nil {
		t.ExpireTime = e.ExpireTime
	}
	if e.Password != nil {
		t.Password = *e.Password
	}
	return t
}

func toDomainList(rows []*ent.Team) []*team.Team {
	out := make([]*team.Team, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out
}
