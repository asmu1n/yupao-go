package team

import (
	"context"
	"time"
)

// Repository 队伍与成员关系持久化端口。
type Repository interface {
	// CreateTeamWithLeader 创建队伍并写入队长成员关系（事务）。
	CreateTeamWithLeader(ctx context.Context, t *Team, leaderID int64) (int64, error)
	GetByID(ctx context.Context, id int64) (*Team, error)
	Update(ctx context.Context, t *Team) error
	// SoftDeleteTeam 逻辑删除队伍。
	SoftDeleteTeam(ctx context.Context, id int64) error
	// SoftDeleteTeamAndMembers 解散：逻辑删除队伍及全部成员关系（事务）。
	SoftDeleteTeamAndMembers(ctx context.Context, teamID int64) error

	CountCreatedByUser(ctx context.Context, userID int64) (int64, error)
	List(ctx context.Context, q QueryParams, includePrivate bool) ([]*Team, error)
	ListPage(ctx context.Context, q QueryParams, includePrivate bool) ([]*Team, int64, error)

	AddMember(ctx context.Context, userID, teamID int64, joinTime time.Time) error
	// SoftDeleteMember 逻辑删除单条成员关系。
	SoftDeleteMember(ctx context.Context, userID, teamID int64) error
	CountMembers(ctx context.Context, teamID int64) (int64, error)
	CountUserMemberships(ctx context.Context, userID int64) (int64, error)
	HasJoined(ctx context.Context, userID, teamID int64) (bool, error)
	// CountMembersByTeamIDs 返回 teamID -> 人数。
	CountMembersByTeamIDs(ctx context.Context, teamIDs []int64) (map[int64]int, error)
	// JoinedTeamIDs 返回 user 已加入且在给定列表中的队伍 ID 集合。
	JoinedTeamIDs(ctx context.Context, userID int64, teamIDs []int64) (map[int64]struct{}, error)
	// ListTeamIDsByUser 用户加入的全部队伍 ID（未删除）。
	ListTeamIDsByUser(ctx context.Context, userID int64) ([]int64, error)
	// ListMembersByTeamOrdered 按 id 升序取成员，用于队长移交。
	ListMembersByTeamOrdered(ctx context.Context, teamID int64, limit int) ([]Member, error)
	// TransferLeaderAndRemoveMember 移交队长并移除原队长成员关系（事务）。
	TransferLeaderAndRemoveMember(ctx context.Context, teamID, oldLeaderID, newLeaderID int64) error
}

// Member 队伍成员关系简要信息。
type Member struct {
	ID     int64
	UserID int64
	TeamID int64
}
