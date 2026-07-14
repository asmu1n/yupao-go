package repository

import (
	"context"

	"yupao-go/ent"
	entuser "yupao-go/ent/user"
	"yupao-go/internal/domain/user"
	"yupao-go/internal/shared/usertype"
)

type EntRepository struct {
	client *ent.Client
}

func New(client *ent.Client) *EntRepository {
	return &EntRepository{client: client}
}

func (r *EntRepository) Create(ctx context.Context, u *user.User) (int64, error) {
	created, err := r.client.User.Create().
		SetUserAccount(u.UserAccount).
		SetPlanetCode(u.PlanetCode).
		SetUserPassword(u.Password).
		SetNillableUsername(u.Username).
		SetNillableAvatarURL(u.AvatarURL).
		SetNillableGender(u.Gender).
		SetNillablePhone(u.Phone).
		SetNillableEmail(u.Email).
		SetNillableTags(u.Tags).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *EntRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
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

func (r *EntRepository) GetByAccount(ctx context.Context, account string) (*user.User, error) {
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

func (r *EntRepository) ExistsByAccount(ctx context.Context, account string) (bool, error) {
	return r.client.User.Query().
		Where(entuser.UserAccountEQ(account)).
		Exist(ctx)
}

func (r *EntRepository) ExistsByPlanetCode(ctx context.Context, code string) (bool, error) {
	return r.client.User.Query().
		Where(entuser.PlanetCodeEQ(code)).
		Exist(ctx)
}

func (r *EntRepository) Update(ctx context.Context, id int64, u *user.User) error {
	_, err := r.client.User.UpdateOneID(id).
		SetUserAccount(u.UserAccount).
		SetPlanetCode(u.PlanetCode).
		SetNillableUsername(u.Username).
		SetNillableAvatarURL(u.AvatarURL).
		SetNillableGender(u.Gender).
		SetNillablePhone(u.Phone).
		SetNillableEmail(u.Email).
		SetNillableTags(u.Tags).
		Save(ctx)
	return err
}

func (r *EntRepository) ListAll(ctx context.Context) ([]*user.User, error) {
	rows, err := r.client.User.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return toDomainList(rows), nil
}

func (r *EntRepository) ListByIDs(ctx context.Context, ids []int64) ([]*user.User, error) {
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
		ID:          e.ID,
		UserAccount: e.UserAccount,
		Password:    e.UserPassword,
		UserStatus:  e.UserStatus,
		UserRole:    e.UserRole,
		CreateTime:  e.CreateTime,
	}
	if e.Username != nil {
		u.Username = e.Username
	}

	if e.AvatarURL != nil {
		u.AvatarURL = e.AvatarURL
	}
	if e.Gender != nil {
		gender := usertype.Gender(*e.Gender)
		u.Gender = &gender
	}
	if e.Phone != nil {
		u.Phone = e.Phone
	}
	if e.Email != nil {
		u.Email = e.Email
	}
	if e.PlanetCode != "" {
		u.PlanetCode = e.PlanetCode
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
