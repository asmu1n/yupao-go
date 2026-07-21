package team

import (
	"time"

	"yupao-go/internal/pkg/page"
	"yupao-go/internal/pkg/types"
)

const (
	maxCreateTeams = 5 // 用户最多创建队伍数
	maxJoinTeams   = 5 // 用户最多加入（含创建）队伍数
	maxTeamMembers = 20
	minTeamMembers = 1
)

// QuitOutcome 退出操作在库内的实际结果。
type QuitOutcome int

const (
	// QuitOutcomeLeft 普通成员退出，队伍仍在。
	QuitOutcomeLeft QuitOutcome = iota
	// QuitOutcomeDisbanded 最后一人退出，队伍解散。
	QuitOutcomeDisbanded
	// QuitOutcomeTransferred 队长退出并移交继任者。
	QuitOutcomeTransferred
)

// QuitResult 退出队伍的结构化结果（供审计日志使用）。
type QuitResult struct {
	Outcome     QuitOutcome
	NewLeaderID int64 // QuitOutcomeTransferred 时有效
}

// Team 队伍领域模型。
type Team struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Description *string          `json:"description"`
	MaxNum      int              `json:"maxNum"`
	ExpireTime  *time.Time       `json:"expireTime"`
	UserID      int64            `json:"userId"` // 队长
	Status      types.TeamStatus `json:"status"`
	Password    string           `json:"-"`
	CreateTime  time.Time        `json:"createTime"`
	UpdateTime  time.Time        `json:"updateTime"`
}

// TeamUserVO 列表/详情展示（含创建人与加入态）。
type TeamUserVO struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Description *string          `json:"description"`
	MaxNum      int              `json:"maxNum"`
	ExpireTime  *time.Time       `json:"expireTime"`
	UserID      int64            `json:"userId"`
	Status      types.TeamStatus `json:"status"`
	CreateTime  time.Time        `json:"createTime"`
	UpdateTime  time.Time        `json:"updateTime"`
	CreateUser  *TeamCreatorVO   `json:"createUser,omitempty"`
	HasJoinNum  int              `json:"hasJoinNum"`
	HasJoin     bool             `json:"hasJoin"`
}

// TeamCreatorVO 队伍创建人公开展示信息。
type TeamCreatorVO struct {
	ID        int64          `json:"id"`
	Username  *string        `json:"username,omitempty"`
	AvatarURL *string        `json:"avatarUrl,omitempty"`
	Gender    *types.Gender  `json:"gender,omitempty"`
}

// AddParams 创建队伍。
type AddParams struct {
	Name        string            `json:"name" binding:"required,max=20"`
	Description *string           `json:"description" binding:"omitempty,max=512"`
	MaxNum      int               `json:"maxNum" binding:"required,min=1,max=20"`
	ExpireTime  *time.Time        `json:"expireTime" binding:"required"`
	Status      *types.TeamStatus `json:"status" binding:"omitempty,oneof=0 1 2"`
	Password    *string           `json:"password" binding:"omitempty,max=32"`
}

// UpdateParams 更新队伍。
type UpdateParams struct {
	ID          int64             `json:"id" binding:"required,gt=0"`
	Name        *string           `json:"name" binding:"omitempty,max=20"`
	Description *string           `json:"description" binding:"omitempty,max=512"`
	ExpireTime  *time.Time        `json:"expireTime"`
	Status      *types.TeamStatus `json:"status" binding:"omitempty,oneof=0 1 2"`
	Password    *string           `json:"password" binding:"omitempty,max=32"`
}

// JoinParams 加入队伍。
type JoinParams struct {
	TeamID   int64   `json:"teamId" binding:"required,gt=0"`
	Password *string `json:"password"`
}

// QuitParams 退出队伍。
type QuitParams struct {
	TeamID int64 `json:"teamId" binding:"required,gt=0"`
}

// DeleteParams 解散队伍。
type DeleteParams struct {
	ID int64 `json:"id" binding:"required,gt=0"`
}

// QueryParams 通用队伍查询条件
type QueryParams struct {
	page.PageRequest
	ID          *int64            `form:"id" json:"id"`
	IDList      []int64           `form:"idList" json:"idList"`
	SearchText  string            `form:"searchText" json:"searchText"`
	Name        string            `form:"name" json:"name"`
	Description string            `form:"description" json:"description"`
	MaxNum      *int              `form:"maxNum" json:"maxNum"`
	OwnerID     *int64            `form:"ownerId" json:"ownerId"`
	Status      *types.TeamStatus `form:"status" json:"status" binding:"omitempty,oneof=0 1 2"`
}

// MyCreateQueryParams 我创建队伍查询条件。
type MyCreateQueryParams struct {
	Status      *types.TeamStatus `form:"status" json:"status" binding:"omitempty,oneof=0 1 2"`
	SearchText  string            `form:"searchText" json:"searchText"`
	Name        string            `form:"name" json:"name"`
	Description string            `form:"description" json:"description"`
	MaxNum      *int              `form:"maxNum" json:"maxNum"`
}

// MyJoinQueryParams 我加入队伍查询条件。
type MyJoinQueryParams struct {
	Status      *types.TeamStatus `form:"status" json:"status" binding:"omitempty,oneof=0 1 2"`
	SearchText  string            `form:"searchText" json:"searchText"`
	Name        string            `form:"name" json:"name"`
	Description string            `form:"description" json:"description"`
	MaxNum      *int              `form:"maxNum" json:"maxNum"`
}
