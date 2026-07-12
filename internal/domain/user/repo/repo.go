package repo

import (
	"context"

	"yupao-go/ent"
	entuser "yupao-go/ent/user"
	"yupao-go/internal/domain/user"
)

type UserRepository struct {
	client *ent.Client
}

func New(client *ent.Client) *UserRepository {
	return &UserRepository{client: client}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) (int64, error) {
	created, err := r.client.User.Create().
		SetNillableUsername(u.Username).
		SetNillableUserAccount(u.UserAccount).
		SetNillableAvatarURL(u.AvatarURL).
		SetNillableGender(u.Gender).
		SetUserPassword(u.Password).
		SetNillablePhone(u.Phone).
		SetNillableEmail(u.Email).
		SetNillablePlanetCode(u.PlanetCode).
		SetNillableTags(u.Tags).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	row, err := r.client.User.Query().
		Where(entuser.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *UserRepository) GetByAccount(ctx context.Context, account string) (*user.User, error) {
	row, err := r.client.User.Query().
		Where(entuser.UserAccountEQ(account)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *UserRepository) ExistsByAccount(ctx context.Context, account string) (bool, error) {
	return r.client.User.Query().
		Where(entuser.UserAccountEQ(account)).
		Exist(ctx)
}

func (r *UserRepository) ExistsByPlanetCode(ctx context.Context, code string) (bool, error) {
	return r.client.User.Query().
		Where(entuser.PlanetCodeEQ(code)).
		Exist(ctx)
}

func (r *UserRepository) Update(ctx context.Context, id int64, u *user.User) error {
	_, err := r.client.User.UpdateOneID(id).
		SetNillableUsername(u.Username).
		SetNillableUserAccount(u.UserAccount).
		SetNillableAvatarURL(u.AvatarURL).
		SetNillableGender(u.Gender).
		SetNillablePhone(u.Phone).
		SetNillableEmail(u.Email).
		SetNillablePlanetCode(u.PlanetCode).
		SetNillableTags(u.Tags).
		Save(ctx)
	return err
}

func (r *UserRepository) ListAll(ctx context.Context) ([]*user.User, error) {
	rows, err := r.client.User.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return toDomainList(rows), nil
}

func (r *UserRepository) ListByIDs(ctx context.Context, ids []int64) ([]*user.User, error) {
	rows, err := r.client.User.Query().
		Where(entuser.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return toDomainList(rows), nil
}

func toDomain(e *ent.User) *user.User {
	u := &user.User{
		ID:         e.ID,
		Password:   e.UserPassword,
		UserStatus: e.UserStatus,
		UserRole:   e.UserRole,
		CreateTime: e.CreateTime,
	}
	if e.Username != "" {
		u.Username = &e.Username
	}
	if e.UserAccount != "" {
		u.UserAccount = &e.UserAccount
	}
	if e.AvatarURL != "" {
		u.AvatarURL = &e.AvatarURL
	}
	if e.Gender != 0 {
		u.Gender = &e.Gender
	}
	if e.Phone != "" {
		u.Phone = &e.Phone
	}
	if e.Email != "" {
		u.Email = &e.Email
	}
	if e.PlanetCode != "" {
		u.PlanetCode = &e.PlanetCode
	}
	if e.Tags != "" {
		u.Tags = &e.Tags
	}
	return u
}

func toDomainList(rows []*ent.User) []*user.User {
	result := make([]*user.User, len(rows))
	for i, row := range rows {
		result[i] = toDomain(row)
	}
	return result
}
