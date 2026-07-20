package user

import (
	"context"
	"time"
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
	ListPage(ctx context.Context, pageParams QueryParams) ([]*User, int64, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*User, error)
	ListActiveMatchCandidates(ctx context.Context, afterID int64, limit int, activeSince time.Time) ([]*User, error)
}
