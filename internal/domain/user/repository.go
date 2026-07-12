package user

import "context"

type Repository interface {
	Create(ctx context.Context, u *User) (int64, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByAccount(ctx context.Context, account string) (*User, error)
	ExistsByAccount(ctx context.Context, account string) (bool, error)
	ExistsByPlanetCode(ctx context.Context, code string) (bool, error)
	Update(ctx context.Context, id int64, u *User) error
	ListAll(ctx context.Context) ([]*User, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*User, error)
}
