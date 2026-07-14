package user

import (
	"encoding/json"
	"time"

	"yupao-go/internal/shared/usertype"
)

const (
	RoleDefault = 0
	RoleAdmin   = 1
)

type User struct {
	ID          int64            `json:"id"`
	UserAccount string           `json:"userAccount"`
	Password    string           `json:"-"`
	Username    *string          `json:"username"`
	AvatarURL   *string          `json:"avatarUrl"`
	Gender      *usertype.Gender `json:"gender"`
	Phone       *string          `json:"phone"`
	Email       *string          `json:"email"`
	PlanetCode  string           `json:"planetCode"`
	Tags        *string          `json:"tags"`
	UserStatus  int              `json:"userStatus"`
	UserRole    int              `json:"userRole"`
	CreateTime  time.Time        `json:"createTime"`
}

func (u *User) ParseTags() []string {
	if u.Tags == nil || *u.Tags == "" {
		return nil
	}
	var tags []string
	_ = json.Unmarshal([]byte(*u.Tags), &tags)
	return tags
}

type registerParams struct {
	UserAccount   string `json:"userAccount"   binding:"required,min=4,max=256"`
	UserPassword  string `json:"userPassword"  binding:"required,min=8"`
	CheckPassword string `json:"checkPassword" binding:"required,min=8,eqfield=UserPassword"`
	PlanetCode    string `json:"planetCode"    binding:"required,max=5"`
}

type loginParams struct {
	UserAccount  string `json:"userAccount"  binding:"required,min=4"`
	UserPassword string `json:"userPassword" binding:"required,min=8"`
}
