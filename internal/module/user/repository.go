package user

import (
	"context"
	"time"
	"yupao-go/internal/pkg/page"
)

// Repository 用户持久化端口，由 user/repo 提供 ent 实现。
type Repository interface {
	Create(ctx context.Context, u *User) (int64, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByAccount(ctx context.Context, account string) (*User, error)
	ExistsByAccount(ctx context.Context, account string) (bool, error)
	ExistsByPlanetCode(ctx context.Context, code string) (bool, error)
	Update(ctx context.Context, id int64, u *User) error
	ListAll(ctx context.Context) ([]*User, error)
	ListPage(ctx context.Context, pageParams QueryParams) (*page.PageResponse[*User], error)
	ListByIDs(ctx context.Context, ids []int64) ([]*User, error)
	// ListActiveMatchCandidates 按 ID 游标分批查询活跃匹配候选（status/未删除/有 tags/近 activeSince 有更新）。
	ListActiveMatchCandidates(ctx context.Context, afterID int64, limit int, activeSince time.Time) ([]*User, error)
}
